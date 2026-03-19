# DMX Controller

A DMX lighting controller that consumes messages from a RabbitMQ queue and applies colour effects and transitions to DMX fixtures in real time. It drives a **Eurolite USB DMX 512 PRO MK2** adapter via direct USB (libusb).

---

## Requirements

### libusb

The controller communicates with the DMX adapter through [libusb](https://libusb.info) via the [`gousb`](https://github.com/google/gousb) wrapper. libusb **must be installed on the host system** before building or running.

**macOS**
```bash
brew install libusb
```

**Ubuntu / Debian**
```bash
sudo apt-get install libusb-1.0-0-dev
```

### sudo / USB permissions

The process needs direct access to the USB device.

**macOS** — run with `sudo`. The Apple FTDI kernel driver must also be unloaded so libusb can claim the device:
```bash
sudo kextunload -b com.apple.driver.AppleUSBFTDI
sudo ./dmxcontroller
```
To reload the driver after you're done:
```bash
sudo kextload -b com.apple.driver.AppleUSBFTDI
```

**Linux** — either run with `sudo`, or add a udev rule so your user can access the device without root:
```bash
# /etc/udev/rules.d/99-eurolite-dmx.rules
SUBSYSTEM=="usb", ATTRS{idVendor}=="0403", ATTRS{idProduct}=="6001", MODE="0666", GROUP="plugdev"
```
Reload udev and re-plug the adapter:
```bash
sudo udevadm control --reload-rules && sudo udevadm trigger
```

### RabbitMQ

A running RabbitMQ broker is required. The default connection URL is `amqp://guest:guest@localhost:5672/`.

---

## Building

```bash
go build -o dmxcontroller .
```

> The `-o dmxcontroller` flag is required because the `dmx/` subdirectory would otherwise conflict with the default binary name.

---

## Usage

```bash
sudo ./dmxcontroller [flags]
```

### Flags

| Flag | Default | Description |
|---|---|---|
| `--rabbitmq-url` | `amqp://guest:guest@localhost:5672/` | RabbitMQ connection URL |
| `--queue` | `dmx` | Queue name to consume from |
| `--interval` | `50ms` | Render loop interval (DMX frame rate) |

### Help

```bash
./dmxcontroller --help
```

---

## Message format

Publish a JSON message to the configured queue:

```json
{"start_address": 11, "light_type": "pst10", "action": "green"}
```

| Field | Description |
|---|---|
| `start_address` | 1-based DMX channel of the fixture's first channel |
| `light_type` | Fixture model key (see below) |
| `action` | Colour or effect to apply (see below) |

### Supported fixtures and actions

#### Eurolite LED PST-10 QCL Spot — `pst10` (9 channels)

| Action | Type | Description |
|---|---|---|
| `red` | transition | Fade all channels to red (200 ms) |
| `green` | transition | Fade all channels to green (200 ms) |
| `blue` | transition | Fade all channels to blue (200 ms) |
| `white` | transition | Fade all channels to white (200 ms) |
| `warm` | transition | Fade to soft warm white — red + white mix (200 ms) |
| `off` | transition | Fade all channels to zero (200 ms) |
| `rainbow` | infinite | Continuously cycle through all hues (~30 fps) |

#### Eurolite LED B-40 Laser — `b40` (22 channels)

The fixture has three independent LED groups. Colour actions apply the same value to all three groups simultaneously.

| Action | Type | Description |
|---|---|---|
| `red` | transition | All groups → red (300 ms) |
| `green` | transition | All groups → green (300 ms) |
| `blue` | transition | All groups → blue (300 ms) |
| `white` | transition | All groups → white (300 ms) |
| `warm` | transition | All groups → warm white (300 ms) |
| `off` | transition | All groups → off (300 ms) |
| `rainbow` | infinite | All groups cycle through hues in sync (~30 fps) |
| `strobe` | infinite | All groups flash white via per-group strobe channels (~12 Hz) |
| `default` | infinite | Blue sweeps across groups 1 → 2 → 3 → 1 with 2.5 s hold and 2.5 s crossfade |

**Transition actions** fade smoothly from the current channel state to the target. The final state persists until a new action arrives — no blackout on completion.  
**Infinite actions** run continuously until a new action is applied to the same address.

---

## Architecture

```
main.go                          cobra CLI setup, flag validation
runner.go                        RunController, AMQP consumer loop
testb40.go                       standalone B-40 test tool (go run testb40.go)
dmx/
  dmx_usb.go                     USB DMX adapter — OpenUSB, SendFrame, Close
internal/
  models/message.go              DMXMessage (JSON payload)
  effects/
    effect.go                    Effect, Fixture, EffectTicker, ChannelWriter interfaces
    helpers.go                   NewTransition, NewThrottledTicker, HSV→RGB
    manager.go                   Manager — tick-driven render loop
    devices/
      pst10.go                   Eurolite PST-10 effects
      b40.go                     Eurolite B-40 effects
```

### How the render loop works

There are **no per-effect goroutines**. All effects are driven by a single render loop that ticks at `--interval`:

1. For every active fixture, the loop calls `effect.Tick(writer)` — effects update their own channel buffer in place.
2. All channel buffers are composed into a single 512-byte DMX universe.
3. `SendFrame` is called once per tick.

Effects implement `EffectTicker.Tick(w ChannelWriter) bool`. Transition effects return `false` when complete (their final buffer state persists — no blackout). Infinite effects always return `true`. Effects that need a lower visual frame rate (e.g. rainbow, strobe, blue-cycle) use `NewThrottledTicker` to skip render ticks internally while still being called by the shared loop.

The only lock in the system is `Manager.mu`, held briefly during each tick to protect the active-effect map from concurrent `Apply` calls arriving from the AMQP consumer goroutine.

### Testing a fixture directly

`testb40.go` is a standalone tool for sending raw DMX frames to the B-40 without RabbitMQ. It is excluded from normal builds via a `//go:build ignore` tag.

```bash
sudo go run testb40.go
```

Edit the `channels` array at the top of the file to change what is sent. The program runs a 40 Hz refresh loop and sends a blackout frame on Ctrl+C.
