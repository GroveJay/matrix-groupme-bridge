# matrix-groupme-bridge
A Matrix-GroupMe puppeting bridge.

## Prior Art

### Faye clients:

* [karmanyaahm/wray](https://github.com/karmanyaahm/wray), a fork of 
* [autogrow/wray](https://github.com/autogrow/wray) which is itself a fork of
* [pythonandchips/wray](https://github.com/pythonandchips/wray)

None of these had a websocket transport. The existing model didn't fit the async push/pull nature of websockets so the code is heavily modified to accomodate it.

Another heavy pass of YAGNI was also applied and I attempted to model the websocket loop based on what the Groupme webapp does.

### GroupMe Golang client

* [densestvoid's groupme repository](https://github.com/densestvoid/groupme) (for freshness)
* [karmanyaahm's fork](https://github.com/karmanyaahm/groupme) for the `IndexRelations` and `real-time` components

The `real-time` components are enhanced to (attempt to) gracefully stop polling loops, attempt reconnection, and restart polling after successful reconnection.

### Prior Bridges

* [beeper/groupme](https://github.com/beeper/groupme), a fork of
* [karmanyaahm/matrix-groupme-go](https://github.com/karmanyaahm/matrix-groupme-go) which forked from an old
* [mautrix/whatsapp](https://github.com/mautrix/whatsapp)
* [mautrix/meta](https://github.com/mautrix/meta/blob/main/pkg/msgconv/media.go) for a modern bridge example

## Architectural Issues

The main issue with improving the bridge is that groupme has a normal HTTP API and a Websocket "subscription" API.
Certain things are only achievable through each of these and to get certain notifications you need to subscribe to individual channels on a per-room basis.
I think this will prove to be prohibitive if this was somehow running for thousands of users. It'd be akin to hosting that many websocket connections at once.
In any case, that limitation brought this bridge to have a very small feature-set as described on the roadmap.

A little of this code was re-used from [mautrix/meta's msgconv/media.go](https://github.com/mautrix/meta/blob/main/pkg/msgconv/media.go) and [beeper/groupme's groupmeext/message.go](https://github.com/beeper/groupme/blob/master/groupmeext/message.go) and modified to remove facebook/meta specific logic.

## Documentation
Setup and usage instructions are similar to those located on [docs.mau.fi]. Some quick links:

[docs.mau.fi]: https://docs.mau.fi/bridges/go/meta/index.html

* [Bridge setup](https://docs.mau.fi/bridges/go/setup.html?bridge=meta)
  (or [with Docker](https://docs.mau.fi/bridges/general/docker-setup.html?bridge=meta))
* Basic usage: [Authentication](https://docs.mau.fi/bridges/go/meta/authentication.html)

## Features & Roadmap
[Roadmap.md](Roadmap.md) contains a general overview of what is supported by the bridge.

