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

| `light_type` | Fixture | Channels | Actions |
|---|---|---|---|
| `pst10` | Eurolite LED PST-10 QCL Spot | 9 | `red` `green` `blue` `white` `warm` `off` `rainbow` |
| `b40` | Eurolite LED B-40 Laser | 22 | `red` `green` `blue` `white` `warm` `off` `rainbow` `strobe` |

**Transition actions** (`red`, `green`, `blue`, `white`, `warm`, `off`) fade smoothly to the target colour over 200 ms.  
**Infinite actions** (`rainbow`, `strobe`) run continuously until a new action is applied to the same address.

---

## Architecture

```
main.go                          cobra CLI setup, flag validation
runner.go                        RunController, AMQP consumer loop
dmx/
  dmx_usb.go                     USB DMX adapter — OpenUSB, SendFrame, Close
internal/
  models/message.go              DMXMessage (JSON payload)
  effects/
    effect.go                    Effect, Fixture, ChannelWriter, DMXDevice interfaces
    helpers.go                   Transition interpolation, HSV→RGB
    manager.go                   EffectManager — per-slot buffers + render loop
    devices/
      pst10.go                   Eurolite PST-10 effects
      b40.go                     Eurolite B-40 effects
```

### How the render loop works

Each active effect runs in its own goroutine and writes only to an isolated slot buffer (one per fixture). A single render goroutine ticks at `--interval`, reads every slot buffer, composes a complete 512-channel universe, and calls `SendFrame` once per tick. No two effect goroutines ever contend on the same lock.
