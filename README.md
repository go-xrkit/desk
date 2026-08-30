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
them, captured and drawn as FLAT screens on a band at eye level. The keyboard
turns the band; one key promotes a screen to fill the view.

**One screen is one full view.** Each virtual display is created at exactly one
eye's resolution — the most the glasses can show at once — and is given exactly
the arc that eye can see. Looking straight at a screen shows it edge to edge, at
one source pixel per output pixel.

**How many screens is a choice, and it stops at nine.** Nothing in the geometry
supplies a ceiling: the screens are flat, so the angles are only a scroll
coordinate and the band is as long as it needs to be — the plan would spread
forty over the turn without complaint. So the limit is *decided*, at
`desk.MaxScreens` = **9**, where three things agree: the gallery is three columns
wide, so nine fills three rows of three exactly and every screen keeps its column
as the desk grows; each screen costs a display to create and a stream to capture,
linearly; and past nine a person stops holding a map of where anything is, which
is the whole point of a fixed arrangement. A bigger number is *clamped* when a
program composes a plan and *refused, with the ceiling named*, when it comes from
a settings file — a clamp is right for code and wrong for a line somebody wrote.

The field of view is **reported, not required**. What fills the glasses is one
source pixel per panel pixel, which needs the panel's resolution and nothing
else, so a headset nobody has heard of runs at the right size and an unknown
field of view prints as unknown rather than as `0.00°`.

## Why the keyboard

The Beast, the Luma Ultra and the XREAL One series all do their 3DoF anchoring
**inside the glasses**, which is also why their motion sensors are not offered
to the host. So head movement is theirs and the ribbon is yours: the two compose
instead of fighting. Turning the band is a key press, and it is deliberate.

### The keys

In the window, and in the glasses:

| key | on the ribbon | in the screen gallery | in the application gallery |
|---|---|---|---|
| `←` `→` | turn the band | move the selection — it wraps, because the ribbon is a circle | move the selection, the same way |
| `↑` `↓` | nothing: a band has no rows | move a row — it clamps, because the fold has no seam | move a row, the same way |
| `Enter` | — | go to the highlighted screen, the short way round | **put the highlighted application on the screen the band is showing** |
| `Space` / `f` | fill the view with the focused screen | — | — |
| `g` | show every screen at once | put the ribbon back exactly as it was | — |
| `a` | show what is RUNNING | — | put it away |
| `x` | one application per screen, in order | — | the same |
| `Tab` / `c` | show the next source here (⌃⌥⌘C from anywhere) | — | — |
| `Escape` / `q` | quit | quit — a mode you cannot leave is a trap | quit |

The two galleries answer different questions. The screen one is *which desktop
am I looking at*; the application one is *what is open, and where is it* — a
grid of everything with a window, each tile saying which screen it is on. The
band keeps its focus underneath it, and that is what makes `Enter` mean "here":
no number to type at a picture you may not be able to see.

`x` hands out one screen per application, in order, up to the ribbon's count.
Anything past the last screen is **left where it is** rather than wrapped onto a
screen that already has one: two windows on one screen hides one of them, and a
person who pressed one key cannot be expected to guess which.

Placing an application needs the **Accessibility** grant, and says so plainly
when it has not got it. It goes through the same code as a `place` block in the
settings file, so the live path and the start-up path cannot drift apart.

### And from anywhere else

The point of a desk in glasses is that you are *using* the screens on it. So the
keys are claimed **system-wide** and work while another application has the
keyboard — which is not a convenience here but the whole design: the desk's own
window is deliberately passive and never takes the keyboard from what is running
on the screens.

| | |
|---|---|
| `⌃⌥⌘←` `⌃⌥⌘→` | turn the band |
| `⌃⌥⌘↑` | show the SCREENS |
| **`⌃⌥⌘↓`** | **show what is RUNNING on them** |
| `⌃⌥⌘↩` | choose: the highlighted screen, or the highlighted application onto the screen in front |
| `⌃⌥⌘A` | the same, for a keyboard where it arrives |
| **`⌃⌥⌘X`** | **one application per screen** |
| **`⌃⌥⌘⇥`** | **what this screen shows — including a mirror of the Mac's own display** |
| `⌃⌥⌘M` | bring the pointer to the screen being looked at |
| `⌃⌥⌘-` `⌃⌥⌘=` | move the band away, and back |
| `⌃⌥⌘[` `⌃⌥⌘]` | flatten the screens, turn them |
| `⌃⌥⌘S` | the settings |
| `⌃⌥⌘⎋` | quit |

macOS only, for now: this goes through Carbon's `RegisterEventHotKey`, which
asks for **no permission at all** — no accessibility prompt, no input
monitoring. On Linux and Windows the keys work in the window and the run says
so. `-no-global` leaves them alone.

**They are not always the keys you get.** xrdesk falls back — Shift, then
Control, then both — and prints whatever it landed on at start-up, because it
has to: of the three ways a shortcut can already be taken, two are detectable
and one is not.

The third one is why the applications are on an ARROW. `⌃⌥⌘A` was granted
without complaint and never delivered a single press: an application's own menu
key is invisible to every check there is, so the only symptom is a key that does
nothing. Arrows are not claimed quietly. An application's own menu
key is invisible to everything. The band was on `⌥⌘←`/`⌥⌘→` until somebody who
had learnt the desk pressed `⌃⌥⌘←` and got nothing: one prefix for all of them
is worth more than two keys saved. Those two also register without complaint and are
also Safari's tab navigation — while xrdesk runs, it wins them, and Safari
quietly stops seeing them. That is the trade a global shortcut is; it is
printed rather than hidden.

## The band is a circle; the desktop is a line

The band follows the pointer: move the mouse onto another of the desk's screens
and the picture turns to that screen. Nothing is warped and nothing is
synthesised — it is the person's own mouse, and the picture catches up.

The two disagree at the ends. Pushed to the left of the **first** screen the
pointer is against a wall: the window server clamps it there, the band cannot
follow it anywhere, and the **last** screen is all the way back across every
screen in between. So a push held into the end brings the pointer back at the
other end, and the band with it.

Held, not touched. The left-hand column of pixels on the first screen is a place
a person legitimately goes — a close button lives there — so a wrap that fired on
merely touching the edge would put that column out of reach. `300 ms` of push is
somebody asking for something.

And only where the desktop **really** ends. The screens this program makes sit
beside the ones the machine already has, and the edge between them is the way to
this Mac's own panel: an end with a display beyond it keeps its edge, or the
wrap would take the real screen away. The test for that is geometric — is there
a display that starts further out than this one — so it holds for whatever
arrangement somebody has, on either side.

With one exception, and it is the one that matters on this machine. **Measured**
with the glasses on and six screens up, on 2026-08-29: the glasses report
themselves at `x=-13440` and the ribbon runs `-11520..0`, so the glasses sit
*immediately* to the left of the first screen. A pointer pushed off that edge is
therefore never clamped anywhere — it walks onto the display showing this
program's own window, where it is a pointer on a picture: it can click nothing,
and nothing in the view says where it went. That is the mouse people lose.

So the desk's own screen is not somewhere the pointer can go. It does not hold
an end open — the left edge really is the end of the desktop — and a pointer
that lands on it is brought back **at once**, with no hold, at whichever end it
left from. That screen is identified by its **rectangle** rather than its size:
two identical monitors are the same size and are not in the same place.

## The Mac's own screen, in the glasses

A ribbon position can show one of this Mac's real displays as easily as one of
the displays this program made: they are both in the sources list, and `⌃⌥⌘C`
walks through it. Somebody with one screen in the glasses can therefore see the
screen they already have.

**Usually without asking for it.** Following ends where the ribbon does: a
pointer that leaves every screen a position is showing — onto this Mac's own
panel, which sits right beside the ribbon — is invisible to somebody wearing
the glasses. They have not lost the mouse exactly; they have lost the screen it
is on. So after `700 ms` there, the desk puts that display on the position in
front of them. Crossing a screen to reach another stays ordinary: the hold is
what tells passing through from arriving.

Measured, end to end, with the glasses on:

```
pointer put at 1000,600 — this Mac's own panel
  the pointer is on display 1, which no screen is showing
  screen 1: display 1 (main) — where the pointer went
  1 of this Mac's screens are off while the ribbon shows them
backlight of display 1:  0.41 → 0.00 → 0.41 when the desk stopped
```

Not on `⌃⌥⌘Tab`, which is where the key used to be. A whole session with the
glasses on logged arrows, the gallery and the menu bar, and **not one cycle**:
macOS keeps Command-Tab for its own application switcher whatever else is held
down with it. The registration succeeds and the key never arrives — the third
kind of conflict, undetectable from here.

When that happens, **the panel itself is turned off** and lit again when the
copy leaves the ribbon or the program stops. It is not tidiness. A person
wearing the glasses is looking at the copy; the panel is a second, brighter
copy of private work at reading distance, facing whoever walks past, lit at
full power for nobody.

Turning the backlight off is not the same as covering the screen with a black
window: the framebuffer is untouched, so the picture ON the ribbon does not
change, there is no window for the capture to exclude and no stream to rebuild,
and nothing another program can raise itself above. It goes through
[go-macos/brightness](https://github.com/go-macos/brightness), whose `Dim` reads
the level BEFORE changing it and hands back the way home — which is deferred
here before anything is turned off, so every way out of a session goes through
it.

Three screens are never darkened:

- a display **this program made**, which has no panel behind it;
- the display **the desk itself is on** — a ribbon position can be pointed at
  the glasses' own display, and darkening that one would black out the thing
  being looked at, with the key that undoes it now invisible. It is identified
  by its rectangle: two identical monitors are the same size and are not in the
  same place;
- **every** screen, when the desk is running in a window rather than in the
  glasses, because then the desk is a window on one of them.

`-dim=false` leaves every screen lit.

**And nothing holds a backlight off after the desk stops.** Two ways out were
missing and both were found the hard way, in one session:

- a **panic in the frame loop** is a panic in another goroutine: it kills the
  process without running the deferred call that puts the backlight back. That
  happened, and the report that came back was « j'ai débranché les lunettes car
  j'avais perdu l'accès et il n'y avait pas l'icon dans la tray pour que je coupe
  l'application » — the icon was there, on a panel that had been turned off. A
  crash now stops the desk the ordinary way, with the stack logged, so every
  restore runs;
- **unplugging the glasses** left it drawing for nobody while holding the
  backlight off and six displays that do not exist. The desk now notices its own
  screen has gone and stops itself.

### A screen takes the shape of what it shows

A screen of the band is a whole view of the glasses, so they are all one shape —
until a position mirrors a display this program did not make. This Mac's panel
is 2056x1329, a ratio of **1.547** against the band's **1.778**, and asked for a
1920x1080 frame of it ScreenCaptureKit letterboxes: measured, **124 flat columns
down each side**. Those bands are then part of the picture, on a screen that
cannot be told to drop them.

Changing the Mac's resolution does not help, which was worth measuring before
building anything: its **sixty display modes are 1.547 and 1.600 and nothing
else**. There is no 16:9 mode to put a MacBook panel into.

So the screen takes the shape instead — same height as its neighbours, narrower
— and the ribbon already knew how to place that: a screen's span on the circle
comes from its aspect ratio, which is what keeps its pixels square. The capture
is asked for at the source's own shape (`1670x1080`, **0 flat columns**), the
band gives that screen its own arc, and the gallery draws it narrower inside a
cell of the usual size rather than stretching it across one.

It is driven by the PIXELS, not by whoever opened the capture: what a position
shows changes while the desk runs, and the shape arrives with the first frame.

A band that cannot hold the shape says so and stays as it was — four screens of
the eye's own shape already take 343° of the available 360°, so a *wider* screen
often does not fit at all.

**This is also where a crash came from**, with the glasses on:

```
panic: slice bounds out of range [:6684] with capacity 6680
```

6680 is 1670 pixels of BGRA. A turned panel gathers its columns one at a time
out of the source row, and the fan was still using the band's 1920. The fan now
reads each screen's own width — and `Canvas.Slant` treats a column outside the
source as background rather than as a place to read from, because a renderer
that can be crashed by a source of the wrong size will be.

## How it is put together

| | |
|---|---|
| [`go-xrkit/xrkit`](https://github.com/go-xrkit/xrkit) | `glasses` names the headset; `ribbon` places screens on the band and composites them by yaw |
| [`go-macos/virtualdisplay`](https://github.com/go-macos/virtualdisplay) | creates the displays macOS extends onto |
| [`go-macos/screencapture`](https://github.com/go-macos/screencapture) | streams their pixels |
| [`go-widgets`](https://github.com/go-widgets) | the window, and every pixel of interface |

There is no distortion table, and no panorama. The screens used to be wrapped on
a cylinder and unwrapped again through a per-pixel warp, which cost 2.8 ms of a
16.6 ms frame to make something whose whole purpose is to look flat — and drew
the screen you look at straight on with a bow in it, arguing with the depth the
glasses already present. Worn once, that settled it. A yaw is now what it looks
like: a horizontal offset into a band of flat pictures, which costs nothing.


### The screens only exist while it runs

macOS lists them, by name, as `XR desk 1` … `XR desk n` — in System Settings ▸
Displays, in `system_profiler SPDisplaysDataType`, and to anything else that
enumerates displays. They sit to the LEFT of the main screen, at negative x, in
ribbon order.

They are created at start and removed at exit, deliberately: a virtual display
that outlives its process is a display somebody has to remove by hand. So an
empty display list means one of two things, and the program now says which:

* **it is not running** — nothing was created, nothing remains;
* **it never got that far** — with no display to show a desk on, `xrdesk` stops
  before creating anything, and says so:

```
glasses: no display matches "VITURE Beast"; attached: "Built-in Retina Display" 2056x1329 (primary)
nothing was created and nothing on this Mac was changed.
  the virtual screens exist only while xrdesk runs, and only alongside a display to show them on
  plug the glasses in, or name one of the displays above with -screen
```

⚠ **Releasing a display is asynchronous.** `virtualdisplay.Close` returns in
microseconds; macOS keeps listing the display for up to **1.9 s** (six of them,
macOS 26.6.2). `Screens.Close` therefore waits — measured, not slept — so that
"released" and "gone from the list" are the same moment for anything that looks
next: the settings phase, which gives the screens back before opening its
window, or a person reading System Settings straight after quitting. Without
that wait, six screens that were already dead still read as a leak; the
integration test fails naming each one if the wait is removed.

### Which glasses are these

Two questions, and they are not the same one: *is a headset attached*, and
*which display is it*.

The bus answers the first. `deskcheck` opens with it:

```
bus
  3318:043e "XREAL 1S" -> XREAL 1S (USB product)

displays
  "Odyssey G95NC" 7680x2160 (primary)
```

That is a real reading. A headset can be plugged in, powered and enumerated
with **no DisplayPort lane at all** — a port carrying USB 2 only, or one whose
bandwidth a large monitor has already taken. It is then present in every sense
except the one that shows a picture, and *no glasses* and *no video* are
different problems that deserve different words.

A USB product id is also the **stronger** evidence of the two. A display name is
whatever the panel put in its EDID, and a dock, a capture card or a KVM in the
path can replace it with something generic; a product id names one model, and
for some brands it is the only thing that does.

But the bus never says *which display* the headset is, so the evidence is
applied only where something ties it to one: you named the display, or the
display names a headset itself and the bus is only saying which one. Otherwise
it is reported and not applied. Lending a headset's optics to a monitor renders
everything, in the wrong place, with no symptom.

Reading it opens no device and needs no permission — macOS asks IOKit for what
the kernel cached at enumeration, Linux reads three files under
`/sys/bus/usb/devices`. Windows and Android fall back to the display name.

## Tested against real hardware

This section says what was actually connected to a machine, and what was only
read off a specification sheet. The difference matters: a field of view taken
from a data sheet renders everything in the wrong place if the data sheet is
wrong, and nothing about the picture says so.

| Hardware | What was actually done |
|---|---|
| Apple M4 Max, macOS 26.6.2 (build 25G83) | everything below |
| **VITURE Beast** | connected over DisplayPort, observed presenting a 3840x1080 side-by-side 3D mode and a 1920x1200 2D mode, and **rendered to** |
| **VITURE Luma Ultra** | **enumerated over USB only** — `35ca:1104 "VITURE Luma Ultra XR GLASSES"`. Connected to this machine on 2026-08-26 alongside the XREAL One S, and like it produced **no display at all**: its display name and modes remain unconfirmed |
| **XREAL One S** | **enumerated over USB only** — `3318:043e "XREAL 1S"`, identified by product id, on three different ports across two buses with the XREAL cable. **DisplayPort alternate mode never engaged**: it negotiated USB 2.0 alone and no second display ever appeared at any layer, so its display name, its modes and its rendering are unconfirmed. A VITURE Luma Ultra on the same machine failed the same way, and the system log knew of exactly one DisplayPort connection throughout — the monitor. Two brands failing identically points at the machine, not at either headset |
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

One test makes REAL displays, so it is behind a build tag and an environment
variable — a runner has no window server, and a display left behind would appear
on somebody's desktop:

```
XRDESK_INTEGRATION=1 go test -tags integration -v -run Integration ./...
```

It reads the SYSTEM's display list, not ours: six screens appear there under the
names macOS shows, and are gone by the time `Close` returns. Remove the wait in
`Screens.Close` and it fails naming each screen still listed — which is how the
wait was shown to be doing something.

## Licence

BSD-3-Clause.
