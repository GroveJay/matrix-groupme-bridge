# Features & roadmap

A list of what's working would be much shorter 😂

The bridge works with a single user's API Token from `https://dev.groupmeclient.com/` and subscribes to just that user over a Websocket connection.
That connection pings every 15 seconds to stay alive and reconnects when it recieves a `/meta/connect` packet from GroupMe. This mimics the web UI and seems pretty robust. This means a small subset of notifications are used to update a user's messages and chats.

On the Matrix -> GroupMe side, I only handled basic text messages.

Overall many things might have to change based on a poor understanding of how to better use bridgev2.
