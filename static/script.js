const chat = document.getElementById('chat');
function OnOpen() {
  chat.textContent = '';
}
function Reload() {
  chat.textContent = 'Reloading...';
  window.location.reload();
}
let alert_queue = [];
function ShowNextAlert() {
  let alertMsg = alert_queue[0];
  let html = alertMsg.html;
  let openMillis = 1000;
  let durationMillis = alertMsg.durationMillis;
  let closeMillis = 1000;
  let time = openMillis + durationMillis + closeMillis;

  var audio = new Audio('door-open.wav');
  audio.volume = 0.8;
  audio.play();

  setTimeout(function () {
    var audio = new Audio('door-close.wav');
    audio.volume = 0.8;
    audio.play();
  }, time - closeMillis + 250);

  let contentElement = document.getElementById('alert-content');
  contentElement.innerHTML = html;

  // Wrap each letter into a .letter span
  let nodes_queue = [contentElement];
  let n = 0;
  while (nodes_queue.length > 0) {
    let node = nodes_queue.shift();
    if (node instanceof Text) {
      for (let word of node.data.split(' ')) {
        let wordSpan = document.createElement('span');
        wordSpan.classList.add('word');
        for (let letter of word) {
          let letterSpan = document.createElement('span');
          letterSpan.classList.add('letter');
          letterSpan.textContent = letter;
          ++n;
          wordSpan.appendChild(letterSpan);
        }
        node.before(new Text(' '));
        node.before(wordSpan);
      }
      node.remove();
    } else {
      for (let child of node.childNodes) {
        nodes_queue.push(child);
      }
    }
  }
  anime.timeline({ loop: false })
    .add({
      targets: '#alert-content .letter',
      translateX: [50, 0],
      skewX: [-15, 0],
      opacity: [0, 1],
      easing: "easeOutExpo",
      duration: 3000,
      delay: (el, i) => 50 * i
    });

  let highlightLetterMillis = durationMillis / n;
  let highlightDuration = 20 * highlightLetterMillis;
  anime.timeline({ loop: false }).add({
    targets: '#alert-content .letter',
    marginLeft: [0, 5, 0],
    easing: "easeInOutSine",
    borderColor: ['#ffffff', '#e65a2f', '#ffffff'],
    duration: highlightDuration,
    delay: (el, i) => Math.max(0, highlightLetterMillis * i + 1000 - highlightDuration / 2),
    update: (anim) => {
      for (let animation of anim.animations) {
        if (animation.type != 'css') {
          continue;
        }
        if (animation.property == 'borderColor') {
          animation.animatable.target.style.setProperty('--color', animation.currentValue);
        }
      }
    },
  });

  let alert = document.getElementById('alert');
  alert.style.setProperty('--time', time + 'ms');
  alert.classList.add('animated');
  alert.addEventListener('animationend', function (ev) {
    if (ev.animationName == 'moveDown') {
      alert.classList.remove('animated');
      contentElement.replaceChildren();
      alert_queue.shift();
      if (alert_queue.length > 0) {
        // setTimeout is needed to allow the browser to remove animation from DOM
        setTimeout(ShowNextAlert, 0);
      }
    }
  });
}

function ShowAlert(html, durationMillis) {
  alert_queue.push({
    html: html,
    durationMillis: durationMillis,
  });
  if (alert_queue.length == 1) {
    ShowNextAlert();
  }
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
  if ('avatar_url' in chat_entry) {
    let avatar = document.createElement('img');
    avatar.src = chat_entry.avatar_url;
    avatar.classList.add('avatar');
    text_span.appendChild(avatar);
  }
  if ('twitch_user_id' in chat_entry) {
    chat_log.dataset.twitch_user_id = chat_entry.twitch_user_id;
  }
  if ('author' in chat_entry) {
    chat_log.dataset.author = chat_entry.author;
    text_span.innerHTML += '<strong style="color:' + color + '">' + chat_entry.author + '</strong>: ' + chat_entry.message;
  } else {
    text_span.innerHTML += chat_entry.message;
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
  if ('twitch_user_id' in chat_entry) {
    let ban_button = document.createElement('button');
    let twitch_user_id = chat_entry.twitch_user_id;
    let user_name = chat_entry.author;
    ban_button.textContent = '💀';
    ban_button.title = 'Ban ' + user_name;
    ban_button.onclick = function () {
      ban_button.innerHTML = 'Ban <strong>' + user_name + '</strong>? ✅';
      ban_button.title = 'Are you sure you want to ban ' + user_name + '?';
      ban_button.onclick = function () {
        ws.send(JSON.stringify({ call: 'BanTwitch', args: [twitch_user_id, user_name] }));
      };
    };
    control_panel.appendChild(ban_button);
  }
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