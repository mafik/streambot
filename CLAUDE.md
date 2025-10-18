# StreamBot

## Project Overview

StreamBot is a Go-based streaming automation bot for Twitch/YouTube live streaming. It provides:
- Aggregated chat from multiple platforms (Twitch, YouTube, Discord)
- High-quality text-to-speech (TTS) for chat messages
- On-stream overlays (gaze tracking, chat display, alerts)
- Automated service management (OBS, VLC, etc.)
- Social media notifications (Twitter/X, Bluesky)
- Moderation tools and control panel

## Building & Running

```bash
go build .
.\streambot.exe
```

The application requires various secrets and external services to be configured. See [README.md](README.md) for detailed setup instructions.

## Architecture

### Core Entry Point
- **main.go** - Main event loop, chat processing, network setup

### Channel-Based Concurrency
The application uses Go channels for inter-component communication:
- `TTSChannel` - Queue for text-to-speech messages
- `AudioPlayerChannel` - Audio playback queue
- `TwitchHelixChannel` - Twitch API operations
- `YouTubeBotChannel` - YouTube API operations
- `MainChannel` - Main thread message processing

### Key Data Structures

#### ChatEntry (main.go)
Core message structure that flows through the system:
- Contains message content, author info, platform-specific IDs
- Methods: `TryTTS()`, `DeleteUpstream()`
- Used across all chat integrations

#### User (user.go)
Unified user representation across platforms:
- Links Twitch, YouTube, Discord accounts
- Manages user preferences (TTS voice selection)
- Ticket-based authentication system

#### Alert (main.go)
On-stream notification structure:
- HTML content for display
- Optional callback functions
- Processed through TTS and audio pipeline

## File Organization

### Chat Platform Integrations
- **twitch_eventsub.go** - Twitch EventSub WebSocket for real-time events
- **twitch_helix.go** - Twitch Helix API for moderation/management
- **youtube_chat.go** - YouTube live chat polling
- **discord.go** - Discord bot integration

### Audio & TTS
- **tts.go** - Text-to-speech using AllTalk TTS server
- **audio_player.go** - Audio playback management
- **muted.go** - Microphone muting detection for TTS pausing

### External Services
- **obs.go** - OBS Studio automation and control
- **vlc_monitor.go** - VLC media player monitoring
- **gaze.go** - Tobii eye tracker integration (requires C++ helper)
- **barrier_monitor.go** - Barrier screen-sharing monitor for scene switching

### Social Media
- **twitter.go** - Twitter/X API for stream notifications
- **bluesky.go** - Bluesky social media integration

### Web Interface
- **webserver.go** - WebSocket server and REST API
- **static/overlay.html** - OBS overlay (loaded as browser source)
- **static/chat.html** - Chat display interface
- **static/script.js** - Frontend JavaScript
- **static/style.css** - Styling

### Utilities
- **user.go** - User management and persistence
- **fs_utils.go** - File system utilities
- **dirs.go** - Directory management
- **windows.go** - Windows-specific functionality
- **ssh.go** - SSH connectivity

## Configuration & Secrets

All API credentials are stored in the `secrets/` directory (not in version control):

### Twitch
- `secrets/twitch_client_id.txt`
- `secrets/twitch_client_secret.txt`
- `secrets/twitch_access_token.txt`
- `secrets/twitch_refresh_token.txt`

### YouTube
- `secrets/youtube_api_key.txt`
- `secrets/youtube_client_secret.json`
- `secrets/youtube_token.json`

### Discord
- `secrets/discord_token.txt`
- `secrets/discord_channel.txt`

### Social Media
- `secrets/twitter_*` (OAuth tokens)
- `secrets/bsky_login.txt`
- `secrets/bsky_password.txt`

### Other
- `secrets/obs_password.txt`
- `secrets/users.json` - User data persistence

## Development Patterns

### Adding New Chat Platforms
1. Create new Go file (e.g., `newplatform.go`)
2. Implement connection management and event handling
3. Convert platform events to `ChatEntry` structs
4. Send to `MainChannel` for processing
5. Add platform-specific user fields to User struct in user.go

### Adding New TTS Voices
1. Add voice files to `static/voices/`
2. Update voice selection UI in `static/script.js`
3. Voice files are referenced by name in User.Voice field

### Adding New Alerts
1. Create `Alert` struct with HTML content
2. Submit to `TTSChannel` for processing
3. Audio files stored in `static/`

### Modifying Web Interface
- HTML: `static/chat.html`, `static/overlay.html`
- JavaScript: `static/script.js`
- CSS: `static/style.css`
- Backend: `webserver.go` (WebSocket calls and REST endpoints)

## Service Integration Pattern

Each external service follows this pattern:
1. **Connection Management** - Establish and maintain connections with retry logic
2. **Event Processing** - Convert service events to internal format (ChatEntry)
3. **API Operations** - Queue operations through service-specific channels
4. **Error Handling** - Graceful degradation with backoff strategies

## External Dependencies

Required for full functionality:
- **AllTalk TTS** - Text-to-speech server (required for TTS)
- **OBS Studio** - Streaming software (required for overlay)
- **Tobii Eye Tracker** - Hardware + SDK (optional, for gaze tracking)
- **Barrier** - Virtual KVM (optional, for multi-screen scene switching)
- **VLC** - Media player (optional, for music track display)

## Common Tasks

### Reading Chat Logs
- Chat history stored in `chat_log.txt` (JSON lines)
- Last 20 messages loaded on startup
- `ReadLastChatLog()` in main.go

### User Authentication
- Ticket-based system in user.go
- Users type `!login <ticket>` in chat to link accounts
- Tickets invalidated after use

### Message Moderation
- Delete via `ChatEntry.DeleteUpstream()`
- Deletes from all platforms (Twitch, YouTube, Discord)
- Uses platform-specific APIs

### TTS Processing
- Language detection (English/Polish) via lingua-go
- Non-English messages auto-deleted with reminder
- TTS paused when microphone is muted in OBS
- Voice selection per-user

## Logging & Debugging

- Terminal output uses color coding (`chat_color`, `warn_color` in main.go)
- Chat logs: `chat_log.txt`
- Each service has independent error handling
- Use `go run .` for development with live output

## Security Considerations

- Never commit files in `secrets/` directory
- API tokens stored in separate files
- User authentication via secure ticket system
- WebSocket connections validate user permissions
- Admin vs. viewer access levels

## Platform-Specific Notes

### Windows
- Primary development platform
- Some functionality may require adaptation for Linux/macOS
- Windows-specific code in `windows.go`

### OBS Integration
- Requires browser source pointing to `static/overlay.html`
- Scene named "Main" with image source "Gaze" for eye tracking
- Microphone input must be named "Mic" for TTS pausing

### Network Configuration
- Webserver runs on port 3447
- Control panel available from local network
- Public IP detection for external access

## Known TODOs (from README.md)

- Automatic shutdown of OBS, VLC & AllTalk on exit
- Move VLC music to local machine
- Timing out users
- Banning users on YouTube
- Viewer count display
- Auto-ban regexps
- Animated avatars for viewers
- Mastodon notifications
- YouTube subscriptions alerts
- GitHub sponsors alerts

## Code Style Notes

- Goroutines for each service integration
- Channel-based communication prevents race conditions
- Consistent error logging with colored terminal output
- Service patterns: connection → event processing → channel communication → error recovery

## Working with This Codebase

When making changes:
1. Review existing patterns in similar files
2. Maintain channel-based architecture
3. Follow error handling conventions
4. Test with multiple platforms simultaneously
5. Ensure graceful degradation when services are unavailable
6. Add appropriate logging with color coding
7. Update user.go for cross-platform user features

This is a personal project tightly coupled with specific hardware/network setup. Changes may require adaptation to your environment.
