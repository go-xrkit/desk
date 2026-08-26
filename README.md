<p align="center"><img src="https://raw.githubusercontent.com/go-xrkit/brand/main/social/go-xrkit.png" alt="go-xrkit" width="720"></p>

# go-xrkit/desk

[![CI](https://github.com/go-xrkit/desk/actions/workflows/ci.yml/badge.svg)](https://github.com/go-xrkit/desk/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/go-xrkit/desk.svg)](https://pkg.go.dev/github.com/go-xrkit/desk)
[![coverage](https://img.shields.io/badge/coverage-100%25%20portable%20logic-brightgreen)](https://github.com/go-xrkit/desk/actions/workflows/ci.yml)
[![license](https://img.shields.io/badge/license-BSD--3--Clause-blue)](LICENSE)

Several computer screens, floating on a 360° ribbon inside AR glasses, scrolled
from the keyboard.

Pure Go, `CGO_ENABLED=0`, no vendor SDK.

## What it is for

XR glasses show one screen. This puts a ring of them around you: real virtual
displays that macOS extends the desktop onto, so ordinary applications run on
them, captured and drawn as curved screens on a band at eye level. The keyboard
turns the band; one key promotes a screen to fill the view.

**One screen is one full view.** Each virtual display is created at exactly one
eye's resolution — the most the glasses can show at once — and is given exactly
the arc that eye can see. Looking straight at a screen shows it edge to edge, at
one source pixel per output pixel.

For a VITURE Beast in its side-by-side 3D mode that works out as six screens of
1920x1080, each spanning 51.57°. For a Luma Ultra or an XREAL One S, seven of
46.06°. The numbers are not configured: they follow from the headset's published
optics.

## Why the keyboard

The Beast, the Luma Ultra and the XREAL One series all do their 3DoF anchoring
**inside the glasses**, which is also why their motion sensors are not offered
to the host. So head movement is theirs and the ribbon is yours: the two compose
instead of fighting. Turning the band is a key press, and it is deliberate.

### The keys

In the window, and in the glasses:

| key | on the ribbon | in the gallery |
|---|---|---|
| `←` `→` | turn the band | move the selection — it wraps, because the ribbon is a circle |
| `↑` `↓` | nothing: a band has no rows | move a row — it clamps, because the fold has no seam |
| `Enter` | — | go to the highlighted screen, the short way round |
| `Space` / `f` | fill the view with the focused screen | — |
| `g` | show every screen at once | put the ribbon back exactly as it was |
| `Tab` / `c` | show the next source here | — |
| `Escape` / `q` | quit | quit — a mode you cannot leave is a trap |

### And from anywhere else

The point of a desk in glasses is that you are *using* the screens on it. So
three of those keys are claimed **system-wide** and work while another
application has the keyboard:

| | |
|---|---|
| `⌥⌘←` `⌥⌘→` | turn the band |
| `⌥⌘Space` | show every screen at once |

macOS only, for now: this goes through Carbon's `RegisterEventHotKey`, which
asks for **no permission at all** — no accessibility prompt, no input
monitoring. On Linux and Windows the keys work in the window and the run says
so. `-no-global` leaves them alone.

**They are not always the keys you get.** `⌥⌘Space` is the Finder's search
window on a stock macOS, so xrdesk falls back — Shift, then Control, then
both — and on this machine ends up with `⌥⇧⌘Space`. Whatever it lands on is
printed at start-up, because it has to be: of the three ways a shortcut can
already be taken, two are detectable and one is not. An application's own menu
key is invisible to everything. `⌥⌘←`/`⌥⌘→` register without complaint and are
also Safari's tab navigation — while xrdesk runs, it wins them, and Safari
quietly stops seeing them. That is the trade a global shortcut is; it is
printed rather than hidden.

## How it is put together

| | |
|---|---|
| [`go-xrkit/xrkit`](https://github.com/go-xrkit/xrkit) | `glasses` names the headset and derives its field of view; `ribbon` places screens on the band and composites them by yaw; `warp` turns the panorama into two eyes |
| [`go-macos/virtualdisplay`](https://github.com/go-macos/virtualdisplay) | creates the displays macOS extends onto |
| [`go-macos/screencapture`](https://github.com/go-macos/screencapture) | streams their pixels |
| [`go-widgets`](https://github.com/go-widgets) | the window, and every pixel of interface |

The renderer never rebuilds its distortion table. Building one costs 56.5 ms and
there are 16.6 ms in a frame — but on an equirectangular panorama a yaw is
exactly a horizontal shift, so the yaw is applied when the screens are
composited into the panorama, where it costs nothing at all.

## Tested against real hardware

This section says what was actually connected to a machine, and what was only
read off a specification sheet. The difference matters: a field of view taken
from a data sheet renders everything in the wrong place if the data sheet is
wrong, and nothing about the picture says so.

| Hardware | What was actually done |
|---|---|
| Apple M4 Max, macOS 26.6.2 (build 25G83) | everything below |
| **VITURE Beast** | connected over DisplayPort, observed presenting a 3840x1080 side-by-side 3D mode and a 1920x1200 2D mode, and **rendered to** |
| **VITURE Luma Ultra** | **enumerated over USB only** — `35ca:1104 "VITURE Luma Ultra XR GLASSES"`. Its video was not connected, so its display name and modes are unconfirmed |
| Samsung Odyssey G95NC, 7680x2160 | used as the working display; it is a genuine 32:9 panel and therefore exercises the one case no arithmetic on panel size can classify |
| Virtual displays | six created at once at 1920x1080, each coming up at exactly the requested size, all removed, the active display list returning to precisely what it was |

**Not yet proven on hardware**: capture of a whole display on macOS, which is
blocked on a Screen Recording permission that could not be granted on the
machine this was built on; capture of an application window IS proven. Every
other pair of glasses in the catalogue is there from published specifications
only, each with its source URL on the entry.

### Send us hardware

We will gladly add and verify a model we can hold. If you want a device
supported, or a figure confirmed rather than quoted, **send us the hardware** and
it will be tested against and listed here. Until then, an unverified entry says
so.

## Platforms

The geometry, the plan and the compositor are portable and have no operating
system in them at all. What differs per platform is capturing pixels and, where
it is possible at all, creating displays.

| | Capture | Virtual displays |
|---|---|---|
| macOS | ScreenCaptureKit | yes, via private CoreGraphics |
| Linux | X11, and Wayland where the compositor offers `wlr-screencopy` | no — capture the displays you have |
| Windows | in progress — enumeration must be added to `go-mswin/win32` first | no — an indirect display driver needs signing |
| Android | `MediaProjection` | **no — settled**; the ribbon carries the phone's own screen |

Android is a different shape from the others. Android hands no drawable surface
to a process that is not the app, and every path to one is behind JNI, which
needs cgo. So an Android build is two processes — a small Java host owning the
Activity and the Surface, and an ordinary `CGO_ENABLED=0 GOOS=android` Go binary
speaking to it over a socket, with pixels through a shared buffer. That is the
pattern `go-widgets/android` already proved with a real installable APK, and it
is the one the capture follows.

On Android the ribbon carries the phone's own screen and our content, not a set
of desktops. That is not a gap to be closed later: Android 15 was asked directly,
and refused four different ways. An app-created virtual display comes back
without `FLAG_TRUSTED`, and launching anything onto it — even the app's OWN
activity — is a `SecurityException`. Asking for `VIRTUAL_DISPLAY_FLAG_TRUSTED`
wants `ADD_TRUSTED_DISPLAY`, whose protection level is `signature`; the flag is
not in the public SDK at all. `VirtualDeviceManager` needs `CREATE_VIRTUAL_DEVICE`,
which is `internal|role` and not grantable even by signature.

What DOES work there, and is worth knowing: on the display the glasses themselves
provide — public and trusted, as any real external display is — an ordinary app
may place other applications with `ActivityOptions.setLaunchDisplayId`. The
refusal is about MAKING a display, not about using the one you were given.

Where virtual displays are not available the ribbon carries the real displays
and windows instead. That is fewer screens, and everything else is identical.

## Tests

```
go test ./...
```

The geometry, the plan and the compositor have no operating system in them, and
they are held at **100 % statement coverage, gated in CI**. The gate selects
files by SHAPE rather than by a list of names -- everything that is not a
platform file (Go's own `_darwin`/`_linux`/`_android`/`_windows`/`_js`/`_other`
suffixes), not a command, and not a `_display.go` -- so a new portable file is
gated the day it is written instead of the day someone remembers to add it.

Playback needs a display, a video file and a pair of glasses, none of which a
runner has, so a total-coverage figure would be a number chosen to pass rather
than a standard. The `_display.go` files are named, not listed, so the exemption
is visible in the file name instead of buried in CI, and they stay deliberately
thin -- wiring pieces that ARE covered.

## Licence

BSD-3-Clause.
