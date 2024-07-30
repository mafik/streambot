const chat = document.getElementById('chat');
function OnOpen() {
  chat.textContent = '';
}
function Reload() {
  chat.textContent = 'Reloading...';
  window.location.reload();
}
let ecg_pings = {
  'Twitch': [],
};
function Ping(component) {
  if (!(component in ecg_pings)) {
    ecg_pings[component] = [];
  }
  ecg_pings[component].push({
    type: 'ping',
    time: Date.now(),
  });
}
function Pong(component) {
  if (!(component in ecg_pings)) {
    ecg_pings[component] = [];
  }
  ecg_pings[component].push({
    type: 'pong',
    time: Date.now(),
  });
  while (ecg_pings[component].length > 10) {
    ecg_pings[component].shift();
  }
}
let canvas = document.getElementById('ecg');
function DrawECG(t) {
  canvas.height = 40 * Object.keys(ecg_pings).length + 30;
  let ctx = canvas.getContext('2d');
  let now = Date.now();
  // ctx.clearRect(0, 0, canvas.width, canvas.height);
  let y = 0;
  ctx.strokeStyle = '#ffffff';
  ctx.font = '40px Belanosima';
  for (let component in ecg_pings) {
    y += 40;
    ctx.lineWidth = 3;
    ctx.strokeStyle = '#ffffff';
    ctx.strokeText(component, 10, y);
    ctx.fillStyle = '#8054a4';
    ctx.fillText(component, 10, y);
    let pings = ecg_pings[component];
    ctx.beginPath();
    let h = y;
    ctx.moveTo(0, h);
    let widthTime = 10000;
    for (let i = pings.length - 1; i >= 0; --i) {
      let entry = pings[i];
      let time = entry.time;
      let x = (now - time) / widthTime * canvas.width;
      if (entry.type == 'ping') {
        ctx.lineTo(x, h + 20);
        ctx.lineTo(x, h);
      } else if (entry.type == 'pong') {
        ctx.lineTo(x, h);
        ctx.lineTo(x, h - 20);
      }
    }
    ctx.lineTo(canvas.width, h);
    ctx.lineWidth = 4;
    ctx.filter = "blur(4px)";
    ctx.strokeStyle = '#a074c4';
    ctx.stroke();
    ctx.filter = "blur(0px)";
    ctx.lineWidth = 1;
    ctx.strokeStyle = '#ffffff';
    ctx.stroke();
  }
  requestAnimationFrame(DrawECG);
}
requestAnimationFrame(DrawECG);
let alert_queue = [];
function ShowNextAlert() {
  let alertMsg = alert_queue[0];
  let html = alertMsg.html;
  let openMillis = 1000;
  let durationMillis = alertMsg.durationMillis;
  let closeMillis = 1000;
  let time = openMillis + durationMillis + closeMillis;

  var audio = new Audio('door-open.wav');
  audio.volume = 0.7;
  audio.play();

  setTimeout(function () {
    var audio = new Audio('door-close.wav');
    audio.volume = 0.7;
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

function AuthorColor(author) {
  let twitch = author.twitch || {};
  let color = twitch.color || 'inherit';
  return color;
}
function AuthorAvatarURL(author) {
  let youtube = author.youtube || {};
  let avatar_url = youtube.avatar_url || '';
  return avatar_url;
}
function AuthorName(author) {
  if ('bot' in author) {
    return '';
  }
  let twitch = author.twitch || {};
  let youtube = author.youtube || {};
  return twitch.name || youtube.name || '';
}
function OnChatMessage(chat_entry) {
  let chat_log = document.createElement('div');
  console.log(chat_entry);
  chat_log.dataset.author = JSON.stringify(chat_entry.author);
  chat_log.classList.add('chat_log');
  let text_span = document.createElement('span');
  let author_name = AuthorName(chat_entry.author);
  text_span.innerHTML = chat_entry.html;

  let control_panel = document.createElement('div');
  control_panel.classList.add('control_panel');

  let can_mute = 'twitch' in chat_entry.author || 'youtube' in chat_entry.author;
  if (can_mute) {
    let mute_button = document.createElement('button');
    mute_button.textContent = '🤫';
    mute_button.title = 'Mute ' + author_name;
    mute_button.onclick = function () {
      ws.send(JSON.stringify({ call: 'ToggleMuted', args: [chat_entry.author] }));
    };
    control_panel.appendChild(mute_button);
  }
  let can_ban = 'twitch' in chat_entry.author;
  if (can_ban) {
    let ban_button = document.createElement('button');
    ban_button.textContent = '💀';
    ban_button.title = 'Ban ' + author_name;
    ban_button.onclick = function () {
      ban_button.innerHTML = 'Ban <strong>' + author_name + '</strong>? ✅';
      ban_button.title = 'Are you sure you want to ban ' + author_name + '?';
      ban_button.onclick = function () {
        ws.send(JSON.stringify({ call: 'Ban', args: [chat_entry.author] }));
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