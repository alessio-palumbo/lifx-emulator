package lan

import (
	"encoding/hex"
	"fmt"
	"strings"

	"lifx-emulator/internal/config"
	"lifx-emulator/internal/emulator"

	"github.com/alessio-palumbo/lifxprotocol-go/gen/protocol/packets"
)

const maxDatagramSize = 4096
const maxLocalPayloadSize = maxDatagramSize - 36

// opaquePayload preserves unknown payload bytes; the existing protocol.Message
// still owns all framing and response correlation metadata.
type opaquePayload struct {
	kind uint16
	data []byte
}

func (p *opaquePayload) PayloadType() uint16            { return p.kind }
func (p *opaquePayload) Size() int                      { return len(p.data) }
func (p *opaquePayload) MarshalBinary() ([]byte, error) { return append([]byte(nil), p.data...), nil }
func (p *opaquePayload) UnmarshalBinary(data []byte) error {
	p.data = append(p.data[:0], data...)
	return nil
}

type compiledPart struct {
	query func() packets.Payload
	data  []byte
}
type localResponse struct {
	requestSize               int
	responseType              uint16
	requestName, responseName string
	parts                     []compiledPart
}

// SetResponses validates and copies local definitions before any packet is
// accepted. Local rules cannot replace generated public packet definitions.
func (r *Router) SetResponses(file config.ResponseFile) error {
	rules := make(map[uint16]localResponse, len(file.Responses))
	for _, definition := range file.Responses {
		if _, exists := packets.Payloads[definition.RequestType]; exists {
			return fmt.Errorf("local request type %d conflicts with the public protocol", definition.RequestType)
		}
		if _, exists := rules[definition.RequestType]; exists {
			return fmt.Errorf("duplicate local request type %d", definition.RequestType)
		}
		if definition.RequestType == 0 || definition.ResponseType == 0 || definition.RequestSize > maxLocalPayloadSize {
			return fmt.Errorf("invalid local request/response type or request size")
		}
		rule := localResponse{requestSize: int(definition.RequestSize), responseType: definition.ResponseType, requestName: definition.RequestName, responseName: definition.ResponseName}
		size := 0
		for _, part := range definition.Parts {
			if (part.Query == "") == (part.Hex == "") {
				return fmt.Errorf("each local response part must specify either query or hex")
			}
			if part.Query != "" {
				var factory func() packets.Payload
				for kind, name := range payloadNames {
					if name == part.Query {
						factory = packets.Payloads[kind]
						break
					}
				}
				if factory == nil || factory().Size() != 0 || !strings.Contains(part.Query, "Get") {
					return fmt.Errorf("local query %q must name a generated public Get with no payload", part.Query)
				}
				rule.parts = append(rule.parts, compiledPart{query: factory})
			} else {
				data, err := hex.DecodeString(part.Hex)
				if err != nil {
					return fmt.Errorf("invalid local response hex: %w", err)
				}
				size += len(data)
				if size > maxLocalPayloadSize {
					return fmt.Errorf("local response exceeds %d bytes", maxLocalPayloadSize)
				}
				rule.parts = append(rule.parts, compiledPart{data: data})
			}
		}
		rules[definition.RequestType] = rule
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.localResponses = rules
	return nil
}

// Caller holds r.mu. Public query parts evaluate the same already-evaluated
// virtual device used by normal Get replies, rather than captured device data.
func (r *Router) localReply(v *emulator.VirtualDevice, p packets.Payload) ([]packets.Payload, error) {
	rule, exists := r.localResponses[p.PayloadType()]
	if !exists {
		return nil, nil
	}
	if p.Size() != rule.requestSize {
		return nil, fmt.Errorf("local request size %d; expected %d", p.Size(), rule.requestSize)
	}
	var data []byte
	for _, part := range rule.parts {
		if part.query != nil {
			states := r.respond(v, part.query(), false)
			if len(states) != 1 {
				return nil, fmt.Errorf("local query part must produce exactly one State reply")
			}
			bytes, err := states[0].MarshalBinary()
			if err != nil {
				return nil, fmt.Errorf("local query part: %w", err)
			}
			data = append(data, bytes...)
		} else {
			data = append(data, part.data...)
		}
		if len(data) > maxLocalPayloadSize {
			return nil, fmt.Errorf("local response exceeds %d bytes", maxLocalPayloadSize)
		}
	}
	return []packets.Payload{&opaquePayload{kind: rule.responseType, data: data}}, nil
}

// Caller holds r.mu. No undocumented names or IDs are built into the binary.
func (r *Router) payloadName(kind uint16) string {
	if name := payloadNames[kind]; name != "" {
		return name
	}
	if rule, ok := r.localResponses[kind]; ok && rule.requestName != "" {
		return rule.requestName
	}
	for _, rule := range r.localResponses {
		if rule.responseType == kind && rule.responseName != "" {
			return rule.responseName
		}
	}
	return fmt.Sprintf("Type%d", kind)
}
