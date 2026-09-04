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

## 3D on anything the glasses are showing

`-3d`, or "3D on" in the menu, turns whatever is on the ribbon into a stereo
pair — a browser, a terminal, a film in a window. It is a toggle, not a mode:
the picture is flat until it is asked for, and flat again the moment it is
turned off.

With `-depth-model` naming a Core ML depth model, the depth comes from a real
network on the Neural Engine and both views from compute kernels on the GPU
([go-xrkit/depth3d](https://github.com/go-xrkit/depth3d)) — about four tenths
of a millisecond of processor time a frame, which is what leaves the rest of
the machine to the desk. Without one, depth is guessed from the picture
itself: it needs nothing, and is visibly worse. The log says which.

It is refused, with a reason, on a display showing one eye — there is nothing
to convert to. And a frame the converter refuses falls back to the flat
picture rather than to black: a viewer who sees the depth go away knows what
happened, and one who sees nothing thinks the desk has crashed.

**The depth is invented.** A captured screen has none of its own, so what this
puts in front of each eye is a guess — a good one on a photograph or a film,
and a confident one about a page of text. That is why it is a switch and not a
default.

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
| `↑` `↓` | nothing: a band has no rows | move a row — and stay in the column, so a column that ends, ends | move a row, the same way |
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
| **`⌃⌥⌘1`…`⌃⌥⌘9`** | **straight to that screen**, from wherever you are |
| **`⌃⌥⌘0`** | **fit**: one screen, the largest these glasses can show it |
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

Every one of these can be MOVED from `desk.hcl`, except quit — a key taken
from the whole machine is a key taken from whatever you were using, so the
default layout is a choice rather than a law. Quit stays put because it is the
way out of a desk that covers a display, and somebody wearing glasses cannot
see the menu bar.

```hcl
shortcut "gallery-open" { keys = "ctrl+alt+cmd+G" }
shortcut "further"     { keys = "ctrl+alt+cmd+Up" }
shortcut "fit"         { keys = "ctrl+alt+cmd+Equal" }
```

⚠ `Equal`, not `=`: the separator between the parts is `-`, so `Minus` is
written as a word and `Equal` follows for the pair to read alike. A name this
file does not know is refused at start-up with the list of the ones it does,
rather than leaving you pressing a key that does nothing.

⚠ And a LETTER can be swallowed. `⌃⌥⌘A` was granted without complaint and never
delivered on the machine this was written for: an application's own menu key is
invisible to everything, and nothing can detect it. An arrow is a key nothing
else claims quietly, which is why the galleries are on them by default.

Nine digits and not ten, because nine is the most screens a desk carries: there
is a key for every one and none spare. All nine are claimed whatever the desk
holds today — the count changes while the session runs, and a key for a screen
that is not there says how many there are rather than doing nothing.

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

## The mouse does not change screens; the keyboard does

A person wearing display glasses sees **one** screen. The desktop under the
pointer is several, most of them invisible, and a pointer that can leave the
visible one is a pointer that can be somewhere its owner cannot look.

That was tried the other way round three times over — the band followed the
pointer, the pointer wrapped round the ends of the band, a display nothing was
showing was fetched onto the screen in front of the viewer when the pointer
wandered onto it — and the report after every one of them was the same: *« j'ai
encore perdu la souris »*. Each mechanism answered a hole left by the one
before, and none of them answered the shape of the problem.

So the pointer stays. It is held to the screen the band is showing, every
frame, and `⌃⌥⌘←` `⌃⌥⌘→` are the way to another one — which brings the pointer
with them, to the **middle** of the screen that has arrived rather than to
whichever edge a clamp would have dragged it to.

It is held to what a position **shows**, not to what this program made: a
position mirroring this Mac's own panel is how somebody reaches their real
desktop, and the pointer has to be able to live there. A position showing
nothing, or a display the machine will not measure, is not a fence at all — the
pointer is left alone rather than held against a rectangle nobody knows.

## The Mac's own screen, in the glasses

**Screen 1 is this Mac's own screen.** Somebody wearing the glasses still has a
Mac in front of them, with a menu bar, a Dock and whatever was already open on
it; reaching it should not mean taking the glasses off, and it should not mean
knowing a key. So one virtual display FEWER is made and the position it would
have taken goes to the machine's own screen — `mirror = false` in the `ribbon`
block turns that off.

Every other position can show one too: they are all in the sources list, and
`⌃⌥⌘C` walks through it.

**And the pointer goes with it.** A position showing this Mac's panel is a
screen of the band like any other, so the pointer is held to *it* while it is
the one in front of you: that is how somebody in the glasses reaches their real
desktop, with the menu bar and everything on it.

Measured, end to end, with the glasses on:

```
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

And a panel this program turned off is put back even when the run that did it
never gets the chance. It writes down what it is about to darken BEFORE
darkening it, and the next start reads that note and lights the panel again:
nothing runs after a `SIGKILL`, so the screen is dark until then -- there is no
way around that -- but it is one start away from being right rather than a
setting somebody has to find in the dark. A panel that is ALREADY brighter than
the note says is left alone, because somebody turned it up in the meantime.

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
  backlight off and six displays that do not exist. The desk looks every second
  for the screen it is on, and stops itself the moment it has gone -- « quand on
  débranche les lunettes il faut rallumer l'écran », and stopping is what does
  that.

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


## Building it

```
go build ./cmd/xrdesk
```

Nothing else to know, and that took a change in another repository to be true:
the item and its menu are [go-widgets/tray](https://github.com/go-widgets/tray)'s,
whose native backends were behind a `tray_native` build tag so the package could
keep a single coverage figure at 100%. A program built the obvious way got a
tray that quietly did not exist. tray v0.6.0 links them by default and gates its
coverage by file shape instead.

That item is also what holds the platform's run loop while the desk waits for a
pair of glasses. Nothing is drawn — and no menu is ever *opened* — without one,
and the window that will own a loop needs a display that is not there yet. So
the tray runs the loop and the waiting happens beside it, which is the way round
`tray` is built for: `Run` when a program has no loop of its own, `Attach` when
it does.

This was learned the long way. The item was mine for a while, built straight on
`NSStatusItem`, and I lent AppKit slices of the main thread to keep it alive.
That drew the icon and never opened its menu, because a menu is not drawn, it is
tracked, and tracking needs the loop running rather than sampled — measured by
clicking the item and counting what appears below the bar: **0 pixels** with the
slices, a menu with the loop.

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


### And a bench that breaks it on purpose

Tests say what functions do to values. Every defect this program had in a week
was about what a **session** does to a **machine**, and every one of them was
found by somebody wearing the glasses and saying so.

`cmd/deskchaos` is the other half. It runs real sessions under real faults — one
left to finish, the glasses unplugged mid-session, killed outright the way a
crash does, a second headset plugged in — and then asks what is still true of
the machine that should not be:

- a display outliving the process that made it;
- a backlight **left dark** — and, for a killed session, whether the next start
  puts it back, because nothing runs after `SIGKILL`;
- the desk's own screen on its own band;
- a panic;
- a session that opened its window and never drew a frame;
- a session whose screen went and never said it was stopping.

It also watches **while** the session runs: a session that misbehaves for ten
seconds and tidies up on the way out leaves nothing behind at all, which is
exactly what a pointer nobody can find looks like.

**It needs no glasses.** A virtual display named from the catalogue is a headset
as far as this program can tell, so the bench makes its own. The only thing it
cannot exercise is the optics.

```
deskchaos -take-the-machine -rounds 20 -budget 90m -report bench.json
```

It refuses to start without `-take-the-machine`, because sessions warp the
pointer sixty times a second, turn a backlight off, make and destroy displays
and move applications between them — and this kills them halfway through on
purpose. `.github/workflows/nightly-bench.yml` is that, on a Mac kept for it.

One thing it taught about Macs rather than about the desk: after several hundred
displays made and destroyed in a day, the window server refuses new ones for the
best part of a minute and takes as long to let old ones go. Rounds after that
report the machine, not the program, so the bench tells the two apart and says
**skipped** rather than counting it.

## Licence

BSD-3-Clause.
