# desk

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
| Windows | in progress | no — an indirect display driver needs signing |
| Android | in progress, via `MediaProjection` | being established — putting other apps on a virtual display appears to need privileges an ordinary APK does not have |

Android is a different shape from the others. Android hands no drawable surface
to a process that is not the app, and every path to one is behind JNI, which
needs cgo. So an Android build is two processes — a small Java host owning the
Activity and the Surface, and an ordinary `CGO_ENABLED=0 GOOS=android` Go binary
speaking to it over a socket, with pixels through a shared buffer. That is the
pattern `go-widgets/android` already proved with a real installable APK, and it
is the one the capture follows.

Where virtual displays are not available the ribbon carries the real displays
and windows instead. That is fewer screens, and everything else is identical.

## Licence

BSD-3-Clause.
