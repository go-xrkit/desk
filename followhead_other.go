// Copyright (c) the go-xrkit authors. All rights reserved.
//
// SPDX-License-Identifier: BSD-3-Clause

//go:build !android

package desk

// FollowHeadByDefault says whether a session should start following the head
// without being asked.
//
// ⛔ FALSE EVERYWHERE ELSE, AND THAT IS NOT TIMIDITY. Following the head opens
// the headset's camera, and a camera coming on by itself is the kind of thing a
// program must be asked to do: on a desktop the ribbon is also perfectly usable
// fixed to the glasses, the shortcut is one key away, and the menu bar row says
// in as many words "Follow my head (camera on)".
//
// Android is the exception because nothing there can ask — see
// followhead_android.go for the measurement.
const FollowHeadByDefault = false
