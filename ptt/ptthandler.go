package ptt

import (
	"errors"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/pion/logging"
	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"github.com/pion/webrtc/v3"
	lutil "github.com/sampleref/gortpdemo/util"
	"io"
	"log"
	"net/http"
	"sync"
	"time"
)

// Peer config
var peerConnectionConfig = webrtc.Configuration{
	ICEServers: []webrtc.ICEServer{
		{
			URLs: []string{"stun:stun.l.google.com:19302"},
		},
	},
	SDPSemantics: webrtc.SDPSemanticsUnifiedPlanWithFallback,
}

var (
	//Media Engine & Setting Engine
	mediaEngine   webrtc.MediaEngine
	settingEngine webrtc.SettingEngine

	//Floor Control
	currentFloorPeer string

	// Local track
	videoTrack     *webrtc.TrackLocalStaticRTP
	audioTrack     *webrtc.TrackLocalStaticRTP
	videoTrackLock = sync.RWMutex{}
	audioTrackLock = sync.RWMutex{}
	// The channel of packets with a bit of buffer
	audioPackets                 = make(chan *rtp.Packet, 60)
	videoPackets                 = make(chan *rtp.Packet, 60)
	localChannelTrackLinked bool = false

	// Websocket upgrader
	upgrader = websocket.Upgrader{}

	//WsConnPeer Map
	ConnMap = make(map[*websocket.Conn]*WsConnPeer)
)

type WsConnPeer struct {
	peerId string
	wsConn *websocket.Conn
	// Peer Connection
	peerConnection *webrtc.PeerConnection
	//API
	api *webrtc.API
}

type JsonMsg struct {
	PeerId       string
	Sdp          string
	IceCandidate struct {
		Candidate     string
		SDPMid        string
		SDPMLineIndex uint16
	}
	FloorControl string
}

const (
	rtcpPLIInterval = time.Second * 1
)

func Initialize() {
	mediaEngine = webrtc.MediaEngine{}
	if err := mediaEngine.RegisterDefaultCodecs(); err != nil {
		panic(err)
	}

	logFactory := logging.NewDefaultLoggerFactory()
	logFactory.DefaultLogLevel = logging.LogLevelDebug
	logFactory.Writer = log.Writer()

	settingEngine = webrtc.SettingEngine{LoggerFactory: logFactory}

	// Create a local video track, all our clients will be fed via this track
	if videoTrack == nil {
		videoRTCPFeedback := []webrtc.RTCPFeedback{{"goog-remb", ""}, {"ccm", "fir"}, {"nack", ""}, {"nack", "pli"}}
		var err error
		videoTrackLock.Lock()
		videoTrack, err = webrtc.NewTrackLocalStaticRTP(
			webrtc.RTPCodecCapability{webrtc.MimeTypeVP8, 90000, 0, "", videoRTCPFeedback},
			"pionv",
			"video",
		)
		videoTrackLock.Unlock()
		lutil.CheckError(err)
	}

	// Create a local audio track, all our clients will be fed via this track
	if audioTrack == nil {
		audioRTCPFeedback := []webrtc.RTCPFeedback{{"transport-cc", ""}}
		var err error
		audioTrackLock.Lock()
		audioTrack, err = webrtc.NewTrackLocalStaticRTP(
			webrtc.RTPCodecCapability{webrtc.MimeTypeOpus, 48000, 2, "minptime=10;useinbandfec=1", audioRTCPFeedback},
			"piona",
			"audio",
		)
		audioTrackLock.Unlock()
		lutil.CheckError(err)
	}
}

func startLocalChannelToTracks() {
	// Asynchronously take all packets in the channel and write them out to tracks
	// Video
	go func() {
		var currTimestamp uint32
		for i := uint16(0); ; i++ {
			packet := <-videoPackets
			// Timestamp on the packet is really a diff, so add it to current
			currTimestamp += packet.Timestamp
			packet.Timestamp = currTimestamp
			// Keep an increasing sequence number
			packet.SequenceNumber = i
			// Write out the packet, ignoring closed pipe if nobody is listening
			if err := videoTrack.WriteRTP(packet); err != nil && !errors.Is(err, io.ErrClosedPipe) {
				panic(err)
			}
		}
	}()
	// Audio
	go func() {
		var currTimestamp uint32
		for i := uint16(0); ; i++ {
			packet := <-audioPackets
			// Timestamp on the packet is really a diff, so add it to current
			currTimestamp += packet.Timestamp
			packet.Timestamp = currTimestamp
			// Keep an increasing sequence number
			packet.SequenceNumber = i
			// Write out the packet, ignoring closed pipe if nobody is listening
			if err := audioTrack.WriteRTP(packet); err != nil && !errors.Is(err, io.ErrClosedPipe) {
				panic(err)
			}
		}
	}()
	localChannelTrackLinked = true
}

func WsConn(w http.ResponseWriter, r *http.Request) {
	// Websocket client
	c, err := upgrader.Upgrade(w, r, nil)
	lutil.CheckError(err)
	// Make sure we close the connection when the function returns
	defer c.Close()
	for {
		var msg JsonMsg
		// Read in a new message as JSON and map it to a Message object
		err := c.ReadJSON(&msg)
		if err != nil {
			log.Printf("error: %v", err)
			delete(ConnMap, c)
			break
		}
		var WsConnPeerObj = ConnMap[c]
		if WsConnPeerObj == nil {
			fmt.Println("New Peer Connection")
			api := webrtc.NewAPI(webrtc.WithMediaEngine(&mediaEngine), webrtc.WithSettingEngine(settingEngine))
			WsConnPeerObj = &WsConnPeer{peerId: msg.PeerId, wsConn: c, api: api}
			ConnMap[c] = WsConnPeerObj
		}
		if msg.Sdp != "" {
			// Create a new RTCPeerConnection
			WsConnPeerObj.peerConnection, err = WsConnPeerObj.api.NewPeerConnection(webrtc.Configuration{})
			lutil.CheckError(err)
			WsConnPeerObj.peerConnection.OnTrack(func(remoteTrack *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
				if remoteTrack.Kind() == webrtc.RTPCodecTypeVideo {
					var lastTimestamp uint32
					// Send a PLI on an interval so that the publisher is pushing a keyframe every rtcpPLIInterval
					go func() {
						ticker := time.NewTicker(rtcpPLIInterval)
						for range ticker.C {
							if WsConnPeerObj.peerId == currentFloorPeer {
								lutil.CheckError(WsConnPeerObj.peerConnection.WriteRTCP([]rtcp.Packet{&rtcp.PictureLossIndication{
									MediaSSRC: uint32(receiver.Track().SSRC())}}))
							}
						}
					}()
					fmt.Println("New Track Fired Video")
					for {
						rtp, _, readErr := remoteTrack.ReadRTP()
						lutil.CheckError(readErr)
						// Change the timestamp to only be the delta
						oldTimestamp := rtp.Timestamp
						if lastTimestamp == 0 {
							rtp.Timestamp = 0
						} else {
							rtp.Timestamp -= lastTimestamp
						}
						lastTimestamp = oldTimestamp

						if WsConnPeerObj.peerId == currentFloorPeer {
							videoPackets <- rtp
						}
					}
				} else {
					fmt.Println("New Track Fired Audio")
					var lastTimestamp uint32
					for {
						rtp, _, readErr := remoteTrack.ReadRTP()
						lutil.CheckError(readErr)
						// Change the timestamp to only be the delta
						oldTimestamp := rtp.Timestamp
						if lastTimestamp == 0 {
							rtp.Timestamp = 0
						} else {
							rtp.Timestamp -= lastTimestamp
						}
						lastTimestamp = oldTimestamp

						if WsConnPeerObj.peerId == currentFloorPeer {
							audioPackets <- rtp
						}
					}
				}
			})

			//Add local source tracks to subscribers to all peers

			// Add local video track
			videoTrackLock.RLock()
			rtpVSender, err := WsConnPeerObj.peerConnection.AddTrack(videoTrack)
			videoTrackLock.RUnlock()
			lutil.CheckError(err)
			// Read incoming RTCP packets
			// Before these packets are retuned they are processed by interceptors. For things
			// like NACK this needs to be called.
			go func() {
				rtcpBuf := make([]byte, 1500)
				for {
					if _, _, rtcpErr := rtpVSender.Read(rtcpBuf); rtcpErr != nil {
						return
					}
				}
			}()

			// Add local audio track
			audioTrackLock.RLock()
			rtpASender, err := WsConnPeerObj.peerConnection.AddTrack(audioTrack)
			audioTrackLock.RUnlock()
			lutil.CheckError(err)
			go func() {
				rtcpBuf := make([]byte, 1500)
				for {
					if _, _, rtcpErr := rtpASender.Read(rtcpBuf); rtcpErr != nil {
						return
					}
				}
			}()

			// Set the remote SessionDescription
			lutil.CheckError(WsConnPeerObj.peerConnection.SetRemoteDescription(
				webrtc.SessionDescription{
					SDP:  string(msg.Sdp),
					Type: webrtc.SDPTypeOffer,
				}))

			// Create answer
			answer, err := WsConnPeerObj.peerConnection.CreateAnswer(nil)
			lutil.CheckError(err)

			// Sets the LocalDescription, and starts our UDP listeners
			lutil.CheckError(WsConnPeerObj.peerConnection.SetLocalDescription(answer))

			// Send server sdp to publisher
			var sdpMsg = JsonMsg{PeerId: msg.PeerId, Sdp: answer.SDP, IceCandidate: struct {
				Candidate     string
				SDPMid        string
				SDPMLineIndex uint16
			}{Candidate: "", SDPMid: "", SDPMLineIndex: 0}}
			lutil.CheckError(c.WriteJSON(sdpMsg))

			// Register for ice candidate generation
			WsConnPeerObj.peerConnection.OnICECandidate(func(candidate *webrtc.ICECandidate) {
				if candidate != nil {
					var iceMsg = JsonMsg{PeerId: msg.PeerId, Sdp: "", IceCandidate: struct {
						Candidate     string
						SDPMid        string
						SDPMLineIndex uint16
					}{Candidate: candidate.ToJSON().Candidate, SDPMid: *candidate.ToJSON().SDPMid, SDPMLineIndex: *candidate.ToJSON().SDPMLineIndex}}
					lutil.CheckError(c.WriteJSON(iceMsg))
				} else {
					fmt.Println("End of ICE Candidates")
				}
			})

			if !localChannelTrackLinked {
				startLocalChannelToTracks()
			}

		} else if &msg.IceCandidate != nil && msg.IceCandidate.Candidate != "" {
			var iceCandidate = webrtc.ICECandidateInit{Candidate: msg.IceCandidate.Candidate, SDPMid: &msg.IceCandidate.SDPMid,
				SDPMLineIndex: &msg.IceCandidate.SDPMLineIndex, UsernameFragment: nil}
			WsConnPeerObj.peerConnection.AddICECandidate(iceCandidate)
		} else if &msg.FloorControl != nil && msg.FloorControl != "" {
			if msg.FloorControl == "REQUEST" {
				if currentFloorPeer == "" {
					currentFloorPeer = msg.PeerId
					var sdpMsg = JsonMsg{PeerId: msg.PeerId, Sdp: "", IceCandidate: struct {
						Candidate     string
						SDPMid        string
						SDPMLineIndex uint16
					}{Candidate: "", SDPMid: "", SDPMLineIndex: 0}, FloorControl: "GRANTED"}
					lutil.CheckError(c.WriteJSON(sdpMsg))
					fmt.Println("Floor Granted")
				} else {
					var sdpMsg = JsonMsg{PeerId: msg.PeerId, Sdp: "", IceCandidate: struct {
						Candidate     string
						SDPMid        string
						SDPMLineIndex uint16
					}{Candidate: "", SDPMid: "", SDPMLineIndex: 0}, FloorControl: "REJECTED"}
					lutil.CheckError(c.WriteJSON(sdpMsg))
					fmt.Println("Floor Rejected")
				}
			} else if msg.FloorControl == "RELEASE" {
				if currentFloorPeer != "" {
					currentFloorPeer = ""
					var sdpMsg = JsonMsg{PeerId: msg.PeerId, Sdp: "", IceCandidate: struct {
						Candidate     string
						SDPMid        string
						SDPMLineIndex uint16
					}{Candidate: "", SDPMid: "", SDPMLineIndex: 0}, FloorControl: "DONE"}
					lutil.CheckError(c.WriteJSON(sdpMsg))
					fmt.Println("Floor Released")
				}
			}
		} else {
			fmt.Println("None of matching msg sdp or ice")
		}
	}
}
