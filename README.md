# StreamBot

This repository contains a stream bot that I use when live coding on Twitch or YouTube.

## Goals

1. [**Eliminate TOIL**](https://sre.google/sre-book/eliminating-toil/). Normal streaming should require minimal manual intervention.
2. **Reduce risk**. Streaming should be resilient to failure and recover quickly if one happens. Streaming should not depend on any external services that can fail.
3. **Increase value**. Try to take the watching experience one level up.

## Features

- Automatic service management
  - Automatic startup of OBS
  - Automatic startup of VLC (for music)
  - Automatic recovery of any failed component with gradual backoff
  - Colored terminal display of logs from all components
  - ***TODO**: automatic shutdown of OBS, VLC & AllTalk on exit*
- On-stream display of real-time gaze position using Tobii eye tracker
- On-stream display of current VLC track
  - ***TODO**: move VLC music to the local machine*

- On-stream display of aggregated chat from Twitch and YouTube
  - Twitch chat client with custom colors & emojis support
  - YouTube chat client with custom avatars & emojis support
  - Chat logging to a file
- High-quality TTS for chat messages with stylized voices
  - Mindful delay of TTS messages while speaking
- Control panel available by opening `control.html` (locally or through `http://IP:3447/control.html`)
  - Button for muting TTS for specific users
  - Button for banning users on Twitch
  - ***TODO**: button for deleting individual messages*
  - ***TODO**: button for timing users out*
  - ***TODO**: button for banning users on YT*
  - ***TODO**: counters with counts of viewers on YT and Twitch*
  - ***TODO**: field for changing stream title on YT and Twitch*
  - ***TODO**: field for changing stream notification on Twitch*
  - ***TODO**: auto-ban regexps*
- ***TODO**: automatic streaming notifications on Twitter and Mastodon*
- On-stream alerts
  - ***TODO**: Twitch follows*
  - ***TODO**: YouTube subscriptions*
  - ***TODO**: GitHub sponsors*
- ***TODO**: OBS scene transition when moving the cursor to a different screen (for drawing in Krita)*
- ***TODO**: Ability for users to change their voice*
- ***TODO**: Animated avatars for stream viewers*

## Warnings

This is my personal project and isn't meant for general use. External packages have been liberally pulled in and may put your machine at risk. Documentation is non-existent (except for what you're reading right now). It's tightly coupled with my home network setup and would require changes to work anywhere else.

That being said, there is nothing stopping you from trying.

A few things to note:

- Most secrets required for API access are stored in the `secrets` directory, which for obvious reasons is not included in this repository. You will have to go over error messages and create the required files.
- TTS depends on the [AllTalk TTS](https://github.com/erew123/alltalk_tts). Go ahead and install it. It's awesome.
- TTS pausing requires the microphone input in OBS to be called "Mic".
- Configure OBS by creating a full-screen browser source that points to the overlay.html file (load it from the local filesystem - not from a server).
- Bot was written with Windows host and Linux target in mind. That being said, it should be relatively easy to adapt it to other setups.
- Tobii gaze tracking requires compiling a C++ helper program. In OBS you should create a scene called "Main" with an image source called "Gaze".

If you're OK with that and want to try it out, you can use these commands as a starting point:

```bash
git clone https://github.com/mafik/streambot.git
cd streambot
go run .
```

If you manage to improve anything, please send a PR!

## Attribution

- https://www.toptal.com/designers/subtlepatterns/