This is a placeholder for a proper architecture documentation.

## Alerts

`Alert` struct can be submitted from any thread, by dropping it into the `TTSChannel`.

TTS Channel synthesizes the wav file and submits it to `AudioPlayerChannel`.

Audio Player waits until `MicIsSilent`.

Audio Player calls the `PrePlay` function, which sends the `ShowAlert` message to WebServer clients, shows the alert in terminal and waits for `ALERT_OPEN_DURATION`.

All WebServer clients animate alert opening for the next `ALERT_OPEN_DURATION`. Alert opening sound is played.

`PrePlay` function ends, and Audio Player starts playing the alert.

All WebServer clients animate alert text for the `playback_duration`.

Audio Player calls the `PostPlay` function, which just waits for `ALERT_CLOSE_DURATION`.

All WebServer clients animate alert closing for the next `ALERT_CLOSE_DURATION`. Alert closing sound is played.

Audio Player continues.
