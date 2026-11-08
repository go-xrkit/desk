# Changelog

This module had **no tags at all** until v0.1.0, which meant anybody importing
it got a pseudo-version of whatever `main` happened to be. Entries below are
written from the code, not from the commit log: 240 pull requests do not make a
history somebody can read.

## Unreleased

### Changed — the curved half of the fold protocol is ON

`SweepCurvedDesks` is `true`. It had been `false` since the protocol learned
about facets, behind about 1300 complaints that the notes below call noise, and
**not one of them was a renderer defect**: every one was a screen's run ending at
the *frame's own* first or last panel, where `Fan.Frame` stops because the next
panel of the chain cannot be drawn. A virtual curl of a hundred degrees seen
from several times its radius shows about a third of itself and turns the rest
away, so demanding the screen whole demanded what the geometry forbids.

Measured across the whole sweep, both directions, before anything was changed:

| | |
|---|---|
| walks that stop on a refused panel | 5740 |
| …where nothing drawable lies beyond, inside the 180° guard | **5740** |
| …where something drawable does (a hole) | **0** |
| …that stop because the reach ran out, or the guard fired early | **0** |

So `checkFrame` exempts a run that ends at the frame's edge, and keeps its
teeth two ways: the rule still examines **45970** run starts and ends away from
those edges — counted, and the sweep fails if that count is ever zero — and
`TestARefusalIsTheBandsEdgeAndNotAGap` holds the premise itself, break-checked
by mutation.

⛔ **The tidy hypothesis was wrong.** `bendFacets` never receives the distance
while its own documentation calls the bend "a radius in multiples of viewing
distance", which would explain the residue growing with distance exactly
(0, 0, 40, 432, 828 at 1x, 1.5x, 2x, 3x, 4x). `ReachDeg` refutes it: the curve
is *virtual* and exists to set head-turn gain, asked for in those words, so
scaling it by distance would silently retune a control somebody set by feel.

Closes [#222](https://github.com/go-xrkit/desk/issues/222) — both halves. The
flat-wide case it opened with was fixed in v0.1.x, measured in `curve_test.go`:
at `toward` 0.25 the unpainted columns went from 1538 to 0.

## v0.4.1 — 2026-11-07

### Fixed — a screen step divided by a facet step, so the head moved the band far too far

Reported from a desk in use: head-following on, *"it jumps from one screen to the
next, it's unusable"*. The journal's own arithmetic on that machine:

```
head: band = 210.0° + head × 46.679
```

`headToBand`'s numerator is a whole **screen** step while its denominator spanned
the focused screen's middle facet to the panel *beside* it — a tenth of a screen
once the screen is curved — so the ratio was multiplied by however finely the
band happened to be cut. The denominator now spans the same thing the numerator
does, carried round the ring when the last screen's next is the first.

Reproduced offline to three decimals, and inert where it should be:

| screens | facets | before | after |
|---|---|---|---|
| 6 × 1920 flat | 6 | 1.238 | 1.238 |
| 3840 + 5 × 1920 flat | 7 | 2.051 | 2.051 |
| 3840 + 5 × 1920 **curved** | 57 | **46.679** | **1.222** |

⚠ The release's own title says "36 times too far", which does not follow from
that table — 46.679 over 1.222 is about 38. The factors above are the measured
ones; the multiplier in the title is not sourced anywhere and should not be
repeated.

## v0.4.0 — 2026-11-07

### Added — three numbers for "the screens jump", and a head that can be followed on the phone

The head path gained an instrument — `HeadBeat` reports what the band did with a
turn, so a complaint about the feel has arithmetic behind it instead of an
opinion. It is what found the defect released as v0.4.1 the same day.

And following your head works on Android: the camera is reached through
`usbfs`, so `-head` on the phone turns on something that answers instead of
refusing. The README carries the run rather than the claim.

## v0.3.0 — 2026-11-01

### Added — xrdesk runs on a phone

Six screens of 1920×1080 on VITURE Beast glasses driven by a Pixel 11 Pro Fold,
`CGO_ENABLED=0`, 312 frames in 5.008 s. Four defects stood between a binary that
cross-compiled and a picture, and not one was visible without the device: it
asked X11 where the glasses were, `window.Open` compiled and failed when called,
a macOS policy waited 15 s a start, and nothing could turn head-following on.

### Changed — the toolchain says which one it was

Go 1.27.3, and `x/net` v0.60.0 for the reachable HTTP/2 advisories.

## v0.2.0 — 2026-10-04

### Changed — Go 1.27.1 is now required, and the CI says which one

`go.mod` asks for `go 1.27.1` and every workflow pins
`go-version: '1.27.1'` instead of `stable`. Raising the directive is a change
in what it takes to BUILD this, so it is a minor and not a patch.

**`stable` had already moved the repository to 1.27 without a commit.** A minor
Go release changes `gofmt` and how coverage is counted, and `stable` is a
channel alias rather than a version: there is nothing to compare, so nothing
opens a pull request and nothing is reviewed. Pinning is what puts each release
in front of a person.

Measured on one commit, both toolchains, before changing anything:

| | `go1.26.6` | `go1.27.1` |
|---|---|---|
| suite | green | green |
| gated files | 100 % | 100 % |
| total statements | 82.0 % | **82.3 %** |
| `gofmt -l` (each from its own GOROOT) | clean | clean |
| `govulncheck` reachable | 0 | 0 |

So 1.27 moves the coverage ratio **up** here. It is the finer block counting,
not more tests: 1.27 breaks a basic block before an instruction that can panic.

⚠ **The security argument is weaker here than elsewhere in this fleet**, and
saying so is the point: `1.26.6` already carried every standard-library fix
this code can reach (#240 raised it there for exactly that reason). This upgrade
buys alignment with the CI, not a smaller vulnerability count.

⚠ **`golang/go#81147`, the loong64 backport, is still open** — read in the
shipped compiler rather than in the issue: `go1.27.1`'s
`LOONG64.rules` still has `(Cvt64Fto32 ...) => (TRUNCDW ...)` and 25 op-codes
declared `reg: fp11`. It cannot reach this repository: a probe with all four
float→int conversions reports all four names, and this code emits **none** of
them on loong64 — and the loong64 lane cross-compiles without ever executing.

## v0.1.2 — 2026-10-04

### Fixed — a flag told you to press keys that were not bound

`-no-global` said it declined «`⌥⌘←/→` and `⌥⌘Space`». The band took the third
modifier months ago, when one prefix for everything beat two keys saved, and
`Space` is bound to **no action at all**. The same stale pair had been copied
into four places — this help, the README table, the documentation site and a
comment — and a flag's help is the worst of them: it is the one piece of
documentation somebody reads with their hands on the keys. It names no key now;
there are forty and they move, the count and where to look do not.

`-wide` said «one screen … instead of the ribbon», which was true until wide
mode stopped clamping the count. It widens every screen and keeps the count.

A test now parses this package with `go/ast` and refuses any **string literal**
naming a combination the code does not grant. Literals rather than the file as
text, because the comments discuss keys that moved and a grep cannot tell
history from a claim about today.

## v0.1.1 — 2026-10-04

### Fixed — a picture of your screens was world-readable

Found by audit, measured on disk, and the inversion is the point: the **journal**
— which only names windows — was already `0600`, while the artefacts it sits
beside were not.

| | was | is |
|---|---|---|
| a snapshot of every display (`⌃⌥⌘P`) | `0644` in a `0755` directory | `0600` in `0700` |
| its note, naming the plan and the focus | `0644` | `0600` |
| a photograph of the room (`⌃⌥⌘L`) | `0644` | `0600` |

The snapshot used `os.Create`, which asks for `0666` and lands on `0644` under
the default umask of 022, so the mode is now **named** rather than left to how a
shell was configured. Both tests ask the filesystem for the mode on the file
rather than reading the argument, because the argument is not what a person can
read.

⚠ **Files already written stay as they were.** `chmod 600` them if you want:
`chmod -R go-rwx ~/Library/Application\ Support/go-xrkit-desk/snapshots`.

## v0.1.0 — 2026-10-04

The first version with a number. `0.x` is the promise: the geometry and the
rendering are measured and worn daily, the **API is not settled**, and a minor
bump may change it.

### What it does

Several real virtual displays on a band inside AR glasses, created at one eye's
resolution each, captured and drawn at **one source pixel per output pixel**, and
turned from the keyboard. macOS extends the desktop onto them, so ordinary
applications run there and the pointer goes with them.

| | |
|---|---|
| screens | 1 to `MaxScreens` = 9, six by default; a decided limit, not a geometric one |
| the band | **flat** — no panorama, no per-pixel warp; `Strip` slides it in row copies |
| turned screens | `Fan` projects facets when a splay is asked for; `-bend` curves a wide one |
| one wide screen | `-wide 6400` with `-reach` for the virtual curve, reached by turning the head |
| galleries | the screens (`⌃⌥⌘F3`) and what is running on them (`⌃⌥⌘F4`), three columns |
| shortcuts | 40, claimed system-wide through Carbon, which asks for no permission at all |
| 3D | a screen given depth, per eye, with the headset asked to switch to its side-by-side mode |
| the room | the headset's camera on a band screen, and a photograph on `⌃⌥⌘L` |

### Platforms

macOS is the only one that runs a desk: it is where virtual displays, screen
capture and Carbon hot keys exist. But every portable package is cross-compiled
for **ten** GOOS/GOARCH pairs in CI — including linux/s390x (big-endian),
linux/loong64 and linux/riscv64 — and the tests run natively on darwin and
linux. So the geometry cannot acquire a little-endian or a 64-bit assumption
without a job going red.

`CGO_ENABLED=0` throughout. No vendor SDK.

### What is measured rather than claimed

- the **portable logic is at 100%** statement coverage, gated in CI by shape
  rather than by a list of file names
- the README's table of shortcuts is **checked against the code** by a test that
  reads the file — it had four wrong rows and sixteen missing before that test
- a **fold protocol** sweeps flat desks for a seam, a hole or a panel out of
  order, at zero complaints
- `govulncheck` reports **no reachable vulnerability**; the `go` directive is
  1.26.6 because four sat in 1.26.4's standard library

### Known limits

- **The curved sweep is off.** `SweepCurvedDesks` is `false`: the protocol
  reports about 1300 complaints on curved desks and the count is not monotone in
  the seam width, so it is noise at that scale rather than a signal. Closing it
  needs an independent geometric reference. See
  [#222](https://github.com/go-xrkit/desk/issues/222).
  *(Resolved — see Unreleased, above. It needed no independent reference: the
  complaints were the protocol asking for something the geometry forbids, and
  `slantOf` could already say so. This line stays as what was believed at this
  release.)*
- **Following your head needs an app bundle.** A camera cannot be opened without
  an `NSCameraUsageDescription`, and a bare executable has no `Info.plist` at
  all. Build one with `go run ./cmd/macapp`; the start-up report says so rather
  than offering a gesture that cannot work.
- **Keyboard scrolling does nothing on a band of one screen**, because the next
  screen of one is itself.
