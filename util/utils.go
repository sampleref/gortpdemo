package util

import (
	"github.com/pion/rtp"
	"log"
	"math/rand"
	"os"
	"time"
)

const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

var seededRand = rand.New(rand.NewSource(time.Now().UnixNano()))

func CheckError(err error) {
	if err != nil {
		panic(err)
	}
}

func StringWithCharset(length int) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}

func ClonePacket(packet *rtp.Packet) *rtp.Packet {
	buf, err := packet.Marshal()
	if err != nil {
		return nil
	}
	var p rtp.Packet
	err = p.Unmarshal(buf)
	if err != nil {
		return nil
	}
	return &p
}

func GetCurrDir() string {
	dir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	return dir
}
