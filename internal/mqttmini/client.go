package mqttmini

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"time"
)

const maxPacketSize = 1024 * 1024

type Client struct {
	Address  string
	Topics   []string
	ClientID string
	OnError  func(error)
}

// Run maintains an MQTT 3.1.1 subscription until ctx is cancelled. It uses
// anonymous authentication because openmiio_agent exposes a local broker with
// no credentials. Connections are retried with bounded exponential backoff.
func (c Client) Run(ctx context.Context, handler func(payload []byte)) {
	backoff := time.Second
	for ctx.Err() == nil {
		err := c.serve(ctx, handler)
		if ctx.Err() != nil {
			return
		}
		if err != nil && c.OnError != nil {
			c.OnError(err)
		}
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		if backoff < 30*time.Second {
			backoff *= 2
		}
	}
}

func (c Client) serve(ctx context.Context, handler func([]byte)) error {
	topics := normalizedTopics(c.Topics)
	if strings.TrimSpace(c.Address) == "" || len(topics) == 0 {
		return errors.New("MQTT address and at least one topic are required")
	}
	topicSet := make(map[string]struct{}, len(topics))
	for _, topic := range topics {
		topicSet[topic] = struct{}{}
	}
	clientID := c.ClientID
	if clientID == "" {
		clientID = "mgl03-homekit-bridge"
	}

	dialer := net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", c.Address)
	if err != nil {
		return fmt.Errorf("connect MQTT %s: %w", c.Address, err)
	}
	defer conn.Close()
	stopClose := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stopClose()

	if err := writePacket(conn, 0x10, connectPayload(clientID)); err != nil {
		return fmt.Errorf("send MQTT CONNECT: %w", err)
	}
	header, payload, err := readPacket(conn)
	if err != nil {
		return fmt.Errorf("read MQTT CONNACK: %w", err)
	}
	if header>>4 != 2 || len(payload) != 2 || payload[1] != 0 {
		return fmt.Errorf("MQTT connection rejected: header=0x%02x payload=%x", header, payload)
	}

	if err := writePacket(conn, 0x82, subscribePayload(1, topics)); err != nil {
		return fmt.Errorf("send MQTT SUBSCRIBE: %w", err)
	}

	awaitingPing := false
	for {
		if err := conn.SetReadDeadline(time.Now().Add(30 * time.Second)); err != nil {
			return err
		}
		header, payload, err = readPacket(conn)
		if err != nil {
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				if awaitingPing {
					return errors.New("MQTT ping timeout")
				}
				if err := writePacket(conn, 0xC0, nil); err != nil {
					return fmt.Errorf("send MQTT PINGREQ: %w", err)
				}
				awaitingPing = true
				continue
			}
			return fmt.Errorf("read MQTT packet: %w", err)
		}
		awaitingPing = false

		switch header >> 4 {
		case 3: // PUBLISH
			topic, message, packetID, qos, err := parsePublish(header, payload)
			if err != nil {
				return err
			}
			if _, ok := topicSet[topic]; ok {
				handler(message)
			}
			if qos == 1 {
				ack := []byte{byte(packetID >> 8), byte(packetID)}
				if err := writePacket(conn, 0x40, ack); err != nil {
					return fmt.Errorf("send MQTT PUBACK: %w", err)
				}
			}
		case 9: // SUBACK
			if len(payload) < 2+len(topics) {
				return fmt.Errorf("MQTT subscription rejected: %x", payload)
			}
			for _, result := range payload[2:] {
				if result == 0x80 {
					return fmt.Errorf("MQTT subscription rejected: %x", payload)
				}
			}
		case 13: // PINGRESP
		default:
			// Other broker control packets are irrelevant for this QoS 0 client.
		}
	}
}

func connectPayload(clientID string) []byte {
	var b bytes.Buffer
	writeUTF8(&b, "MQTT")
	b.WriteByte(4)    // MQTT 3.1.1
	b.WriteByte(0x02) // clean session
	_ = binary.Write(&b, binary.BigEndian, uint16(60))
	writeUTF8(&b, clientID)
	return b.Bytes()
}

func normalizedTopics(topics []string) []string {
	seen := make(map[string]struct{}, len(topics))
	out := make([]string, 0, len(topics))
	for _, topic := range topics {
		topic = strings.TrimSpace(topic)
		if topic == "" {
			continue
		}
		if _, ok := seen[topic]; ok {
			continue
		}
		seen[topic] = struct{}{}
		out = append(out, topic)
	}
	return out
}

func subscribePayload(packetID uint16, topics []string) []byte {
	var b bytes.Buffer
	_ = binary.Write(&b, binary.BigEndian, packetID)
	for _, topic := range topics {
		writeUTF8(&b, topic)
		b.WriteByte(0) // requested QoS 0
	}
	return b.Bytes()
}

func writeUTF8(b *bytes.Buffer, s string) {
	_ = binary.Write(b, binary.BigEndian, uint16(len(s)))
	b.WriteString(s)
}

func writePacket(w io.Writer, header byte, payload []byte) error {
	if len(payload) > maxPacketSize {
		return fmt.Errorf("MQTT packet too large: %d", len(payload))
	}
	packet := []byte{header}
	packet = append(packet, encodeRemainingLength(len(payload))...)
	packet = append(packet, payload...)
	_, err := w.Write(packet)
	return err
}

func readPacket(r io.Reader) (byte, []byte, error) {
	var first [1]byte
	if _, err := io.ReadFull(r, first[:]); err != nil {
		return 0, nil, err
	}
	length, err := decodeRemainingLength(r)
	if err != nil {
		return 0, nil, err
	}
	if length > maxPacketSize {
		return 0, nil, fmt.Errorf("MQTT packet exceeds %d bytes", maxPacketSize)
	}
	payload := make([]byte, length)
	if _, err := io.ReadFull(r, payload); err != nil {
		return 0, nil, err
	}
	return first[0], payload, nil
}

func encodeRemainingLength(n int) []byte {
	var out []byte
	for {
		encoded := byte(n % 128)
		n /= 128
		if n > 0 {
			encoded |= 128
		}
		out = append(out, encoded)
		if n == 0 {
			return out
		}
	}
}

func decodeRemainingLength(r io.Reader) (int, error) {
	multiplier, value := 1, 0
	for i := 0; i < 4; i++ {
		var b [1]byte
		if _, err := io.ReadFull(r, b[:]); err != nil {
			return 0, err
		}
		value += int(b[0]&127) * multiplier
		if b[0]&128 == 0 {
			return value, nil
		}
		multiplier *= 128
	}
	return 0, errors.New("malformed MQTT remaining length")
}

func parsePublish(header byte, payload []byte) (topic string, message []byte, packetID uint16, qos byte, err error) {
	if len(payload) < 2 {
		return "", nil, 0, 0, errors.New("short MQTT PUBLISH packet")
	}
	topicLen := int(binary.BigEndian.Uint16(payload[:2]))
	if topicLen == 0 || len(payload) < 2+topicLen {
		return "", nil, 0, 0, errors.New("invalid MQTT PUBLISH topic")
	}
	topic = string(payload[2 : 2+topicLen])
	pos := 2 + topicLen
	qos = (header >> 1) & 0x03
	if qos > 1 {
		return "", nil, 0, qos, fmt.Errorf("unsupported MQTT QoS %d", qos)
	}
	if qos == 1 {
		if len(payload) < pos+2 {
			return "", nil, 0, qos, errors.New("missing MQTT PUBLISH packet id")
		}
		packetID = binary.BigEndian.Uint16(payload[pos : pos+2])
		pos += 2
	}
	return topic, payload[pos:], packetID, qos, nil
}
