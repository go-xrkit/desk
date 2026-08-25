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

## Licence

BSD-3-Clause.
