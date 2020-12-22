package rtp

import (
	"fmt"
	"github.com/pion/rtcp"
	"github.com/pion/rtp"
	"net"
)

func ReadAndWriteRTP() {
	// Open a UDP Listener for RTP Packets on port 5004
	listener, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 33333})
	if err != nil {
		panic(err)
	}

	// Create remote addr
	var raddr1 *net.UDPAddr
	if raddr1, err = net.ResolveUDPAddr("udp", fmt.Sprintf("127.0.0.1:%d", 44444)); err != nil {
		panic(err)
	}
	var raddr2 *net.UDPAddr
	if raddr2, err = net.ResolveUDPAddr("udp", fmt.Sprintf("127.0.0.1:%d", 55555)); err != nil {
		panic(err)
	}

	// Dial udp
	var senderConn1 *net.UDPConn
	if senderConn1, err = net.DialUDP("udp", nil, raddr1); err != nil {
		panic(err)
	}
	defer func(conn net.PacketConn) {
		if closeErr := conn.Close(); closeErr != nil {
			panic(closeErr)
		}
	}(senderConn1)
	var senderConn2 *net.UDPConn
	if senderConn2, err = net.DialUDP("udp", nil, raddr2); err != nil {
		panic(err)
	}
	defer func(conn net.PacketConn) {
		if closeErr := conn.Close(); closeErr != nil {
			panic(closeErr)
		}
	}(senderConn2)

	defer func() {
		if err = listener.Close(); err != nil {
			panic(err)
		}
	}()

	inboundRTPPacket := make([]byte, 1500)
	// Read RTP packets forever and send them to the WebRTC Client
	fmt.Println("Reading RTP Packets...")
	for {
		n, _, err := listener.ReadFrom(inboundRTPPacket)
		if err != nil {
			fmt.Printf("error during read: %s", err)
			panic(err)
		}
		// Unmarshal the incoming packet
		packet := &rtp.Packet{}
		var rtcpPacket = &rtcp.RawPacket{}
		if err = rtcpPacket.Unmarshal(inboundRTPPacket); err != nil {
			panic(err)
		}
		rtcpPacket.Header()
		if err = packet.Unmarshal(inboundRTPPacket[:n]); err != nil {
			panic(err)
		}
		fmt.Println(packet.SSRC)

		senderConn1.Write(inboundRTPPacket)
		senderConn2.Write(inboundRTPPacket)
	}

}
