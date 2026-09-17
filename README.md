# lifx-emulator

A lightweight Go/Wails desktop testing companion for `lifxlan-go`, `lifx-maestro`, Hikari, and other LIFX LAN clients. It exposes convincing virtual lights on your LAN; it does not emulate complete firmware.

The first launch creates a Color bulb (PID 27), a 16-zone Z strip (PID 32), and a five-device 8×8 Tile chain (PID 55). The desktop can add registry products, edit identity, remove or disable lights, select an IPv4 interface, and display evaluated colors and recent packet activity.

## Development and building

Install Go 1.26+, Node.js 22+, and Wails v2.12 with the native prerequisites described in the [Wails installation guide](https://wails.io/docs/gettingstarted/installation). On macOS, install Xcode Command Line Tools. Linux needs the appropriate GTK/WebKit development packages; Windows needs WebView2.

This development project uses the inspected sibling checkouts through `go.mod` replacements:

```text
personal/
  lifx-emulator/
  lifxlan-go/        # v0.10.0 or newer; pkg/effects inverse physical surface API
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

macOS output is `build/bin/lifx-emulator.app`. The frontend is vanilla JavaScript, canvas, and Vite, keeping the initial UI small. Wails generates its bindings during development/build. No Hikari dependency is used; previews adapt its muted dark presentation, matrix-cell approach, and perceptual brightness curve. All geometry, hidden cells, orientation, and logical color mapping come from `lifxlan-go`.

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

Only configured identity, enabled status, interface, and topology persist. Colors, power, animations, and activity start fresh on launch. Generated targets use a locally administered unicast MAC prefix and cryptographic random bytes. Duplicate configured targets are rejected. Disable a light before editing its serial; labels have the protocol's 32-byte limit.

The registry describes capabilities, not physical matrix dimensions or strip lengths. Choose physical zone count, **send width**, height, and chain length when adding a light. These are virtual-device topology inputs, not copied registry geometry rules. Tile defaults are 8×8 with five chain members; a Candle Color can use 5×6, a Ceiling 8×8, and a Ceiling 13×26 a physical 8×16 layout. The library derives display rows, offsets, hidden cells, and capsule reshaping. JSON supports individual chain `Orientations` (0 upright, 1 upside down, 2 face up, 3 face down, 4 left, 5 right); the creation form applies one orientation to all chain members. Chain members are displayed side by side.

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

On macOS 15 or later, enable lifx-emulator in System Settings → Privacy & Security → Local Network. Click **Request LAN access** in the desktop app to trigger a local-network operation while the app is foregrounded, and allow the macOS prompt. The request connects a UDP socket without sending a datagram; it cannot conclusively report permission status. If macOS previously denied access, enable the app in System Settings and retry discovery. The app includes a local-network usage description. Incoming broadcasts can require permission even when unicast control works. If an independent receiver misses limited broadcasts as your user but receives them under `sudo`, investigate process access policy; root is only a diagnostic comparison, not a normal way to run the desktop app. Recent traffic shows RX requests and TX State writes. Hover a row for the peer IP/port, source ID, sequence, response count, and send error; RX requests that produce no reply are marked. A successful UDP write does not prove client receipt. The socket summary shows RX, decoded, TX, dropped, and send-error counters; hover for the last sender and error. If a packet appears in tcpdump but RX stays unchanged, investigate OS permissions, interface delivery, and firewall rules before changing protocol responses.

Allow inbound UDP 56700 and outbound UDP replies in your firewall. Clients must share a broadcast domain; guest Wi-Fi, access-point isolation, VLAN boundaries, containers, and VPN routing may prevent discovery. A selected interface still uses wildcard socket binding to receive broadcasts, then filters by incoming interface and selects the reply source through IPv4 packet metadata. Unsupported packet-metadata platforms report an interface-selection error; wildcard listening remains available. Another process cannot own the same UDP port. Ephemeral-port tests and headless overrides avoid that conflict, but normal LAN discovery expects port 56700.

Supported queries include service, product/version, host and Wi-Fi firmware, label, device/light power, whole color, legacy and extended multizone state, and matrix chain/64-color state. Multizone and matrix firmware-effect Gets report OFF; effect execution and SetEffect remain unsupported. Location/group and signal queries return static virtual metadata to satisfy `lifxlan-go` classification; the signal value is not a measurement.

Supported visual Sets:

- Whole-light HSBK color, including every zone of strips and all matrix chain members.
- Device power and light power with duration, without destroying stored colors.
- Legacy single-zone/range and extended multizone updates with offsets; `NO_APPLY` buffers, `APPLY` includes buffered changes, and `APPLY_ONLY` flushes without writing packet colors. The applying packet's duration controls the flush.
- Matrix `Set64`, chain index/length, physical x/y and packet row width, clipped partial updates, and hidden physical slots. Only framebuffer 0 is supported.
- Saw, Sine, Half-Sine, Triangle, and Pulse, including optional HSBK masks, period, cycles, pulse skew, and transient completion. Transient effects and Sine/Triangle restore their starting colors; other non-transient effects retain their target colors, following the [LIFX waveform documentation](https://lan.developer.lifx.com/docs/waveforms).

Every accepted Set produces an immediate internal invalidation. The response policy returns newly applied or currently evaluated State messages consistently and ignores `ack_required`/`res_required`. Gets evaluate animations at request time. Unsupported messages are ignored. This deliberately avoids firmware timing and acknowledgment quirks.

## Optional local response rules

For experimental compatibility, load user-supplied replies from `responses.local.json` beside the device configuration. `LIFX_EMULATOR_RESPONSES` overrides that path for desktop and headless; the headless `-responses` flag also selects a file. Restart after editing. The default missing file disables this feature; a missing explicit override or invalid file reports an error.

Local definitions are not bundled into the binary. Keep them outside the repository; the default filename and packet captures are also ignored by Git. This keeps fixtures out of the published source, but does not conceal replies from LAN observers.

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

Each part either marshals a current public State reply from a generated, payload-free Get query, or appends literal local hex bytes. Query parts must produce exactly one State for the addressed product. Parts are concatenated in order. No private packet structures, message IDs, or captured account data are built in. Rules cannot override public request types. Invalid request lengths, unsupported query parts, and replies larger than 4060 payload bytes produce no reply and appear in traffic diagnostics. Unknown packet types without rules likewise appear as unsupported RX entries rather than malformed packets.

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

Canonical HSBK storage is `effects.PhysicalColorState`: all 16 protocol bits are preserved. This is an intentional adjustment to the suggested `[]device.Color` model because `device.NewColor` rounds HSB values. The inverse surface adapter produces lossless `device.Color` display frames. The frontend alone converts HSBK to RGB and applies a square-root brightness/power curve, with Kelvin-aware white rendering.

Transitions store per-zone start/destination/time bounds and evaluate lazily. Interruptions start from the current interpolated color; hue follows the shortest circular path. Power has its own envelope. No transition goroutines are created. One mutex serializes engine access, with no frontend calls under that lock. UI updates coalesce at about 60 Hz during animations or packet activity; idle snapshots run at 4 Hz without emitting unchanged frames. LAN responses never wait for frontend rendering. Recent traffic retains the last 80 routed messages.

## Verification and limits

```sh
gofmt -w main.go cmd internal
go test ./...
go test -race ./...
npm --prefix frontend test
wails build
```

Tests cover lossless immediate colors, transition midpoints/completion/interruption, hue wraparound, independent zones, power envelopes, buffering/offsets, matrix partial updates/chains/orientation/hidden cells/capsule surfaces, all waveforms, completion and masks, evaluated Gets, persistence, and target isolation. UDP tests use ephemeral ports and exercise multiple-device discovery and all three light surfaces. A separate integration test runs the real `lifxlan-go` controller to prove discovery/classification.

This first version does not implement cloud/accounts, switches or hybrid button products, HEV, infrared, diagnostics, firmware effects (Move/Flame/Morph/Sky), scenes, scheduling, arbitrary chain positions, extra framebuffers, strict acknowledgment handling, firmware quirks, or physical device behavior beyond visual color/power. Products with excluded capabilities may still be used for their ordinary light surface; capabilities remain registry-derived. Strip count is limited to 255 for legacy query compatibility; matrices to 4096 physical zones per chain and 16 chain members. Serial uniqueness is guaranteed within configuration; independent emulator hosts use probabilistically unique random targets.
