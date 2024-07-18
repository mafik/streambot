const chat = document.getElementById('chat');
function OnOpen() {
  chat.textContent = '';
}
function Reload() {
  chat.textContent = 'Reloading...';
  window.location.reload();
}
function SetAudioMessage(message) {
  document.getElementById('audio-highlight').textContent = message;
  document.getElementById('audio-fill').textContent = message;
  document.getElementById('audio-shadow').textContent = message;
}
function OnChatMessage(chat_entry) {
  let chat_log = document.createElement('div');
  chat_log.classList.add('chat_log');
  if ('source' in chat_entry) {
    chat_log.classList.add(chat_entry.source.toLowerCase());
  }
  let color = 'inherit';
  if ('author_color' in chat_entry) {
    color = chat_entry.author_color;
  }
  let text_span = document.createElement('span');
  if ('author' in chat_entry) {
    chat_log.dataset.author = chat_entry.author;
    text_span.innerHTML = '<strong style="color:' + color + '">' + chat_entry.author + '</strong>: ' + chat_entry.message;
  } else {
    text_span.innerHTML = chat_entry.message;
  }

  let control_panel = document.createElement('div');
  control_panel.classList.add('control_panel');
  let mute_button = document.createElement('button');
  mute_button.textContent = '🤫';
  mute_button.title = 'Mute ' + chat_entry.author;
  mute_button.onclick = function () {
    ws.send(JSON.stringify({ call: 'ToggleMuted', args: [chat_entry.author] }));
  };
  control_panel.appendChild(mute_button);
  let ban_button = document.createElement('button');
  ban_button.textContent = '💀';
  ban_button.title = 'Ban ' + chat_entry.author;
  ban_button.onclick = function () {
    ws.send(JSON.stringify({ call: 'Ban', args: [chat_entry.author] }));
  };
  control_panel.appendChild(ban_button);
  chat_log.appendChild(control_panel);
  chat_log.appendChild(text_span);

  chat.insertBefore(chat_log, chat.firstChild);
  if (chat.children.length > 20) {
    chat.removeChild(chat.lastChild);
  }
}
function OnMessage(event) {
  let json = JSON.parse(event.data);
  if ('call' in json) {
    let call = json.call;
    let args = json.args || [];
    if (call in window) {
      window[call](...args);
    } else {
      console.error('Unknown call:', call, args);
    }
  }
}
var ws;
function Connect() {
  if (location.host == "" || location.host == "absolute") {
    ws = new WebSocket('ws://localhost:3447/ws');
  } else {
    ws = new WebSocket('ws://' + location.host + '/ws');
  }
  ws.onopen = OnOpen;
  ws.onmessage = OnMessage;
  ws.onclose = OnClose;
}
function OnClose() {
  chat.textContent = 'Connection lost. Reconnecting...';
  setTimeout(Connect, 1000);
}
Connect();

var noSleep = new NoSleep();
noSleep.enable();