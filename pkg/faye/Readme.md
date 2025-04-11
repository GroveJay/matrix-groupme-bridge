# Attribution

Code modified from [karmanyaahm/wray](https://github.com/karmanyaahm/wray), a fork of [autogrow/wray](https://github.com/autogrow/wray) which is itself a fork of [pythonandchips/wray](https://github.com/pythonandchips/wray).

None of these had a websocket transport and the existing adding it didn't fit the async push/pull nature of websockets so the code is heavily modified to accomodate it.

Another heavy pass of YAGNI was also applied and I attempted to model the websocket loop based on what the Groupme webapp does.