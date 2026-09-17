# lifx-emulator

A lightweight virtual LIFX light emulator for testing LIFX LAN clients. Built with Go and a Wails desktop UI, it exposes virtual lights on your network and displays their colors, transitions, and waveforms.

The first launch creates a Color bulb (PID 27), a 16-zone Z strip (PID 32), and a five-device 8×8 Tile chain (PID 55). The desktop can add registry products, edit identity, remove or disable lights, select an IPv4 interface, and display evaluated colors and recent packet activity.

## Development and building

Install Go 1.26+, Node.js 22+, and Wails v2.12 with the native prerequisites described in the [Wails installation guide](https://wails.io/docs/gettingstarted/installation). On macOS, install Xcode Command Line Tools. Linux needs the appropriate GTK/WebKit development packages; Windows needs WebView2.

Development currently requires sibling library checkouts through `go.mod` replacements:

```text
personal/
  lifx-emulator/
  lifxlan-go/        # v0.11.0; pkg/effects inverse physical surface API
  lifxprotocol-go/   # generated packet definitions
  lifxregistry-go/   # generated registry
```

```sh
go mod tidy
npm --prefix frontend ci
wails dev
```

Build the production desktop app:

```sh
wails build
```

Build output is placed in `build/bin/`; on macOS, the desktop bundle is `lifx-emulator.app`. Wails generates frontend bindings during development and builds.

Run without a desktop:

```sh
go run ./cmd/headless
# Log socket delivery counters for discovery troubleshooting (quit desktop first):
go run ./cmd/headless -listen 0.0.0.0:56700 -traffic
# Separate temporary configuration and an alternate port:
go run ./cmd/headless -config /tmp/virtual-lights.json -listen 127.0.0.1:56701
```

The listen override is not persisted. Headless operation needs no Wails runtime or frontend.

## Configuration and topology

Definitions persist atomically in the OS user configuration directory under `lifx-emulator/devices.json`. On macOS this is `~/Library/Application Support/lifx-emulator/devices.json`. `LIFX_EMULATOR_CONFIG` overrides the path for the desktop and default headless runner.

Only configured identity, location/group, enabled status, interface, and topology persist. Colors, power, animations, and activity start fresh on launch. Generated targets use a locally administered unicast MAC prefix and cryptographic random bytes. Duplicate configured targets are rejected. Disable a light before editing its serial; labels have the protocol's 32-byte limit.

All lights in one emulator configuration share a location and group. The listening panel shows both names; use its settings cog to rename them. Each new configuration gets random persisted UUIDs, so independent emulator instances on the same LAN have distinct identities. Renaming keeps the IDs and advances the metadata timestamps; the default group label is the host name. Older configurations receive these fields automatically on their next launch.

The advanced ID fields allow intentional sharing across emulator instances: use the same location UUID to share a location, or the same group UUID to share a group. Matching labels alone do not merge IDs. Headless mode uses the same top-level `Location` and `Group` objects in `devices.json`, each with `ID` (UUID string), `Label` (up to 32 UTF-8 bytes), and `UpdatedAt` (nanoseconds since epoch). Restart after manual edits and update the timestamp when changing metadata.

The registry describes capabilities, not physical matrix dimensions or strip lengths. Choose physical zone count, **send width**, height, and chain length when adding a light. Tile defaults are 8×8 with five chain members; a Candle Color can use 5×6, a Ceiling 8×8, and a Ceiling 13×26 a physical 8×16 layout. The library derives display rows, offsets, hidden cells, and capsule reshaping. JSON supports individual chain `Orientations` (0 upright, 1 upside down, 2 face up, 3 face down, 4 left, 5 right); the creation form applies one orientation to all chain members. Chain members are displayed side by side.

Example additional strip definition:

```json
{
  "Serial": "020001020304",
  "Label": "Test strip",
  "Product": 32,
  "Enabled": true,
  "Zones": 32
}
```

The file root has `Listen` and `Devices` fields. Restart after manually changing the file. Invalid configurations are reported instead of silently replaced.

## LAN behavior

Default UDP listen address is `0.0.0.0:56700`. Discovery returns a service response for each enabled virtual device, with its own target and the request's source/sequence metadata. Direct targets affect only that light; an empty target broadcasts to all applicable enabled lights. All lights share the host address and port.

On macOS 15 or later, allow lifx-emulator in System Settings → Privacy & Security → Local Network. Click **Request LAN access** while the desktop app is foregrounded to trigger the permission prompt. If access was previously denied, enable it in System Settings and retry discovery. Incoming broadcasts can require permission even when unicast control works.

Recent traffic shows incoming requests and outgoing State replies. Hover a row for the peer, source ID, sequence, response count, and send errors. The socket summary reports received, decoded, sent, and dropped packets. A successful UDP write does not prove client receipt. If packets appear in a capture but the receive counter stays unchanged, check OS permissions, interface selection, and firewall rules.

Allow inbound UDP 56700 and outbound UDP replies in your firewall. Clients must share a broadcast domain; guest Wi-Fi, access-point isolation, VLAN boundaries, containers, and VPN routing may prevent discovery. A selected interface still uses wildcard socket binding to receive broadcasts, then filters by incoming interface and selects the reply source through IPv4 packet metadata. Unsupported packet-metadata platforms report an interface-selection error; wildcard listening remains available. On Windows, use `0.0.0.0`; explicit LAN-interface selection is unsupported, while loopback addresses remain available for tests. Another process cannot own the same UDP port. Ephemeral-port tests and headless overrides avoid that conflict, but normal LAN discovery expects port 56700.

Supported queries include service, product/version, host and Wi-Fi firmware, label, device/light power, whole color, legacy and extended multizone state, and matrix chain/64-color state. Multizone and matrix firmware-effect Gets report OFF; effect execution and SetEffect remain unsupported. Location/group queries return the persisted metadata. Signal queries return a static virtual value, not a measurement.

Supported visual Sets:

- Whole-light HSBK color, including every zone of strips and all matrix chain members.
- Device power and light power with duration, without destroying stored colors.
- Legacy single-zone/range and extended multizone updates with offsets; `NO_APPLY` buffers, `APPLY` includes buffered changes, and `APPLY_ONLY` flushes without writing packet colors. The applying packet's duration controls the flush.
- Matrix `Set64`, chain index/length, physical x/y and packet row width, clipped partial updates, and hidden physical slots. Only framebuffer 0 is supported.
- Saw, Sine, Half-Sine, Triangle, and Pulse, including optional HSBK masks, period, cycles, pulse skew, and transient completion. Transient effects and Sine/Triangle restore their starting colors; other non-transient effects retain their target colors, following the [LIFX waveform documentation](https://lan.developer.lifx.com/docs/waveforms).

Every accepted Set produces an immediate internal invalidation. The response policy returns newly applied or currently evaluated State messages consistently and ignores `ack_required`/`res_required`. Gets evaluate animations at request time. Unsupported messages are ignored. This deliberately avoids firmware timing and acknowledgment quirks.

## Optional local response rules

For experimental compatibility, load user-supplied replies from `responses.local.json` beside the device configuration. `LIFX_EMULATOR_RESPONSES` overrides that path for desktop and headless; the headless `-responses` flag also selects a file. Restart after editing. Local rules override bundled defaults individually by request type. A missing default file preserves bundled defaults; a missing explicit override or invalid file reports an error.

Local rules can supplement or override compatibility defaults included in a binary. Keep local definitions outside the repository; local files and packet captures are ignored by Git.

The following uses **synthetic message IDs** and arbitrary bytes, not an additional protocol definition:

```json
{
  "responses": [
    {
      "request_type": 60000,
      "request_size": 0,
      "response_type": 60001,
      "request_name": "LocalQuery",
      "response_name": "LocalState",
      "parts": [
        { "query": "DeviceGetLabel" },
        { "hex": "01020304" }
      ]
    }
  ]
}
```

Each part either marshals a current public State reply from a generated, payload-free Get query, or appends literal local hex bytes. Query parts must produce exactly one State for the addressed product. Parts are concatenated in order. No private packet structures, message IDs, or captured account data are defined in the source. Rules cannot override public request types. Invalid request lengths, unsupported query parts, and replies larger than 4060 payload bytes produce no reply and appear in traffic diagnostics. Unknown packet types without rules likewise appear as unsupported RX entries rather than malformed packets.

The normal protocol library constructs every response header using the virtual target and the request's source/sequence. Literal parts replay **payload bytes only**, never a captured packet header. Rules answer queries; they do not modify identity, ownership, or light state, and do not implement account association.

## Structure and timing

```text
internal/config    registry-backed persisted definitions and topology validation
internal/emulator  physical state, message application, lazy transitions/waveforms
internal/lan       UDP transport, target routing, isolated State response policy
internal/app       device management, persistence, coalesced Wails snapshots
frontend           device controls, canvas surfaces, HSBK-to-display RGB
cmd/headless       standalone UDP runner
```

The Go state engine preserves all 16 bits of each HSBK component. Device geometry and orientation use the library surface mapping. The frontend converts evaluated colors to RGB, including Kelvin-aware whites and a brightness curve that keeps dim colors distinguishable.

Transitions store per-zone start/destination/time bounds and evaluate lazily. Interruptions start from the current interpolated color; hue follows the shortest circular path. Power has its own envelope. No transition goroutines are created. One mutex serializes engine access, with no frontend calls under that lock. UI updates coalesce at about 60 Hz during animations or packet activity; idle snapshots run at 4 Hz without emitting unchanged frames. LAN responses never wait for frontend rendering. Recent traffic retains the last 80 routed messages.

## Verification and limits

```sh
gofmt -w main.go cmd internal
go test ./...
go test -race ./...
npm --prefix frontend test
wails build
```

Tests cover lossless immediate colors, transition midpoints/completion/interruption, hue wraparound, independent zones, power envelopes, buffering/offsets, matrix partial updates/chains/orientation/hidden cells/capsule surfaces, all waveforms, completion and masks, evaluated Gets, persistence, and target isolation. UDP tests use ephemeral ports and exercise multiple-device discovery and all three light surfaces. A client integration test verifies discovery and classification.

This first version does not implement cloud/accounts, switches or hybrid button products, HEV, infrared, diagnostics, firmware effects (Move/Flame/Morph/Sky), scenes, scheduling, arbitrary chain positions, extra framebuffers, strict acknowledgment handling, firmware quirks, or physical device behavior beyond visual color/power. Products with excluded capabilities may still be used for their ordinary light surface; capabilities remain registry-derived. Strip count is limited to 255 for legacy query compatibility; matrices to 4096 physical zones per chain and 16 chain members. Serial uniqueness is guaranteed within configuration; independent emulator hosts use probabilistically unique random targets.
