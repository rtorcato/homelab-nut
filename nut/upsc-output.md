# upsc output reference

Output from `sudo upsc myups` for an APC Back-UPS ES 650G1.

## Quick status

| Variable | Value | Meaning |
|---|---|---|
| `ups.status` | `OL` | On line (mains power OK) |
| `ups.status` | `OB` | On battery (power outage) |
| `ups.status` | `LB` | Low battery (shutdown imminent) |
| `ups.status` | `OL CHRG` | On line and battery charging |
| `ups.status` | `OB LB` | On battery, low — shutdown should trigger now |
| `ups.load` | `29` | UPS is at 29% of its capacity |

---

## Battery

| Variable | Meaning |
|---|---|
| `battery.charge` | Charge percentage (100 = full) |
| `battery.charge.low` | Threshold at which NUT considers battery low and triggers shutdown (default 10%) |
| `battery.charge.warning` | Warning threshold (default 50%) |
| `battery.runtime` | Estimated seconds of runtime remaining at current load (1416 = ~23 min) |
| `battery.runtime.low` | Minimum runtime before shutdown triggers (default 120s = 2 min) |
| `battery.voltage` | Current battery voltage (13.7V — healthy for a 12V lead-acid) |
| `battery.voltage.nominal` | Expected nominal voltage (12V) |
| `battery.type` | Battery chemistry (`PbAc` = sealed lead-acid) |
| `battery.mfr.date` | Battery manufacture date — useful for knowing when to replace |

---

## Input (mains power)

| Variable | Meaning |
|---|---|
| `input.voltage` | Current mains voltage (123.0V) |
| `input.voltage.nominal` | Expected nominal voltage (120V for North America) |
| `input.transfer.high` | Voltage ceiling — above this the UPS switches to battery (139V) |
| `input.transfer.low` | Voltage floor — below this the UPS switches to battery (92V) |
| `input.sensitivity` | How aggressively the UPS reacts to voltage fluctuations (`high` = most sensitive) |

---

## UPS device

| Variable | Meaning |
|---|---|
| `ups.model` | UPS model name |
| `ups.mfr` | Manufacturer |
| `ups.serial` | Serial number |
| `ups.firmware` | UPS firmware version |
| `ups.beeper.status` | Whether the UPS alarm beeper is enabled |
| `ups.delay.shutdown` | Seconds the UPS waits after shutdown command before cutting power (20s) |
| `ups.timer.reboot` | Countdown to reboot (-1 = not active) |
| `ups.timer.shutdown` | Countdown to shutdown (-1 = not active) |

---

## Driver

| Variable | Meaning |
|---|---|
| `driver.name` | NUT driver in use (`usbhid-ups`) |
| `driver.version` | Driver version |
| `driver.parameter.vendorid` | USB vendor ID (`051D` = APC) |
| `driver.parameter.productid` | USB product ID (`0002`) |
| `driver.parameter.pollfreq` | How often (seconds) the driver polls the UPS for a full update (30s) |
| `driver.parameter.pollinterval` | How often (seconds) the driver checks for status changes (2s) |
| `driver.state` | `quiet` = running normally |
| `driver.flag.allow_killpower` | Whether NUT can command the UPS to cut output power (`0` = disabled) |

---

## Shutdown logic

NUT triggers a shutdown when **either** of these thresholds is crossed:

- `battery.charge` drops to or below `battery.charge.low` (10%)
- `battery.runtime` drops to or below `battery.runtime.low` (120s)

To adjust the low-battery threshold:

```bash
# Lower the charge threshold to 20% (more conservative)
sudo upscmd -u admin -p adminpass myups battery.charge.low 20
```

Or set it permanently in `ups.conf`:

```ini
[myups]
    driver = usbhid-ups
    port = auto
    vendorid = 051D
    productid = 0002
    desc = "APC Back-UPS ES 650G1"
    override.battery.charge.low = 20
```
