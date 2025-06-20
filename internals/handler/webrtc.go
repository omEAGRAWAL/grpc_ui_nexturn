// internals/handler/webrtc.go
package handler

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/pion/webrtc/v3"
	"net/http"
)

var peers = map[string]*webrtc.PeerConnection{}

func HandleRTCOffer(c *gin.Context) {
	peerID := uuid.NewString()

	pc, err := webrtc.NewPeerConnection(webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{{URLs: []string{"stun:stun.l.google.com:19302"}}},
	})
	if err != nil {
		c.AbortWithError(500, err)
		return
	}

	dc, _ := pc.CreateDataChannel("grpc", nil)
	attachGRPCHandlers(dc) // see § 2.3

	offer, _ := pc.CreateOffer(nil)
	_ = pc.SetLocalDescription(offer)

	peers[peerID] = pc
	c.JSON(http.StatusOK, gin.H{"id": peerID, "sdp": offer.SDP})
}

type answerPayload struct{ ID, SDP string }

func HandleRTCAnswer(c *gin.Context) {
	var a answerPayload
	if err := c.ShouldBindJSON(&a); err != nil {
		c.AbortWithStatus(400)
		return
	}

	pc := peers[a.ID]
	if pc == nil {
		c.AbortWithStatus(404)
		return
	}

	desc := webrtc.SessionDescription{Type: webrtc.SDPTypeAnswer, SDP: a.SDP}
	if err := pc.SetRemoteDescription(desc); err != nil {
		c.AbortWithError(500, err)
	}
	c.Status(204)
}

type wireMsg struct {
	ID      string          `json:"id"`
	Service string          `json:"service"`
	Method  string          `json:"method"`
	Mode    string          `json:"mode"`    // "" → infer
	Payload json.RawMessage `json:"payload"` // JSON or binary
	Seq     int32           `json:"seq"`     // streaming sequence
}

func attachGRPCHandlers(dc *webrtc.DataChannel) {
	dc.OnMessage(func(m webrtc.DataChannelMessage) {
		var req wireMsg
		if err := json.Unmarshal(m.Data, &req); err != nil {
			return
		}

		go func() {
			// 1. look‑up method in descriptorSet (same code you already have)
			// 2. dial local gRPC server **on the client**? — not here.
			//    The server now sends the CALL to the *browser proxy*.
		}()
	})
}
