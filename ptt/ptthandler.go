package ptt

import (
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/pion/logging"
	"github.com/pion/rtcp"
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
	//API
	api *webrtc.API
	//Floor Control
	currentFloorPeer string

	// Local track
	videoTrack     *webrtc.TrackLocalStaticRTP
	audioTrack     *webrtc.TrackLocalStaticRTP
	videoTrackLock = sync.RWMutex{}
	audioTrackLock = sync.RWMutex{}

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
	rtcpPLIInterval = time.Second * 3
)

func Initialize() {
	mediaEngine := webrtc.MediaEngine{}
	if err := mediaEngine.RegisterDefaultCodecs(); err != nil {
		panic(err)
	}

	logFactory := logging.NewDefaultLoggerFactory()
	logFactory.DefaultLogLevel = logging.LogLevelDebug
	logFactory.Writer = log.Writer()

	settingEngine := webrtc.SettingEngine{LoggerFactory: logFactory}

	api = webrtc.NewAPI(webrtc.WithMediaEngine(&mediaEngine), webrtc.WithSettingEngine(settingEngine))

	// Create a local video track, all our clients will be fed via this track
	if videoTrack == nil {
		videoRTCPFeedback := []webrtc.RTCPFeedback{{"goog-remb", ""}, {"ccm", "fir"}, {"nack", ""}, {"nack", "pli"}}
		var err error
		videoTrackLock.Lock()
		videoTrack, err = webrtc.NewTrackLocalStaticRTP(webrtc.RTPCodecCapability{webrtc.MimeTypeVP8, 90000, 0, "", videoRTCPFeedback}, "video", "pion")
		videoTrackLock.Unlock()
		lutil.CheckError(err)
	}

	// Create a local audio track, all our clients will be fed via this track
	if audioTrack == nil {
		var err error
		audioTrackLock.Lock()
		audioTrack, err = webrtc.NewTrackLocalStaticRTP(
			webrtc.RTPCodecCapability{webrtc.MimeTypeOpus, 48000, 2, "minptime=10;useinbandfec=1", nil},
			"pion",
			"audio",
		)
		audioTrackLock.Unlock()
		lutil.CheckError(err)
	}
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
			WsConnPeerObj = &WsConnPeer{peerId: msg.PeerId, wsConn: c}
			ConnMap[c] = WsConnPeerObj
		}
		if msg.Sdp != "" {
			// Create a new RTCPeerConnection

			WsConnPeerObj.peerConnection, err = api.NewPeerConnection(webrtc.Configuration{})
			lutil.CheckError(err)

			/*tA, err := WsConnPeerObj.peerConnection.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio)
			lutil.CheckError(err)
			tA.Sender().GetParameters().Encodings[0].SSRC = webrtc.SSRC(audioTrackSSRC)

			tV, err := WsConnPeerObj.peerConnection.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo)
			lutil.CheckError(err)
			tV.Sender().GetParameters().Encodings[0].SSRC = webrtc.SSRC(videoTrackSSRC)*/

			WsConnPeerObj.peerConnection.OnTrack(func(remoteTrack *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
				if remoteTrack.Kind() == webrtc.RTPCodecTypeVideo {
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
					rtpBuf := make([]byte, 1400)
					for {
						i, _, err := remoteTrack.Read(rtpBuf)
						lutil.CheckError(err)
						if WsConnPeerObj.peerId == currentFloorPeer {
							videoTrackLock.RLock()
							_, err = videoTrack.Write(rtpBuf[:i])
							videoTrackLock.RUnlock()
							if err != io.ErrClosedPipe {
								lutil.CheckError(err)
							}
						}
					}
				} else {
					fmt.Println("New Track Fired Audio")
					rtpBuf := make([]byte, 1400)
					for {
						i, _, err := remoteTrack.Read(rtpBuf)
						if WsConnPeerObj.peerId == currentFloorPeer {
							lutil.CheckError(err)
							audioTrackLock.RLock()
							_, err = audioTrack.Write(rtpBuf[:i])
							audioTrackLock.RUnlock()
							if err != io.ErrClosedPipe {
								lutil.CheckError(err)
							}
						}
					}
				}
			})

			//Wait Until A Local Video Track is Created
			for {
				videoTrackLock.RLock()
				if videoTrack == nil {
					videoTrackLock.RUnlock()
					//if videoTrack == nil, waiting..
					time.Sleep(100 * time.Millisecond)
				} else {
					videoTrackLock.RUnlock()
					break
				}
			}

			//Add local source tracks to subscribers to all peers

			// Add local video track
			videoTrackLock.RLock()
			_, err = WsConnPeerObj.peerConnection.AddTrack(videoTrack)
			videoTrackLock.RUnlock()
			lutil.CheckError(err)

			//Wait Until A Local Audio Track is Created
			for {
				audioTrackLock.RLock()
				if audioTrack == nil {
					audioTrackLock.RUnlock()
					//if audioTrack == nil, waiting..
					time.Sleep(100 * time.Millisecond)
				} else {
					audioTrackLock.RUnlock()
					break
				}
			}

			// Add local audio track
			audioTrackLock.RLock()
			_, err = WsConnPeerObj.peerConnection.AddTrack(audioTrack)
			audioTrackLock.RUnlock()
			lutil.CheckError(err)

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
