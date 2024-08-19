
function UpdateTabs() {
  var activeFound = false;
  var pagesElement = document.getElementById('pages');
  if (!pagesElement) {
    return;
  }
  var pagesHeight = pagesElement.clientHeight;
  document.querySelectorAll('.tab').forEach(tab => {
    let contentElementID = tab.dataset['tab'];
    let contentElement = document.getElementById(contentElementID);
    contentElement.style.height = pagesHeight + 'px';
    if (tab.classList.contains('active')) {
      activeFound = true;
      contentElement.classList.remove('translate-left');
      contentElement.classList.remove('translate-right');
    } else {
      if (activeFound) {
        contentElement.classList.remove('translate-left');
        contentElement.classList.add('translate-right');
      } else {
        contentElement.classList.add('translate-left');
        contentElement.classList.remove('translate-right');
      }
    }
  });
}
UpdateTabs();

window.onresize = UpdateTabs;

function TabActivate(e) {
  let btn = e.target;
  document.querySelector('.tab.active').classList.remove('active');
  btn.classList.add('active');
  UpdateTabs();
}

document.querySelectorAll('.tab').forEach(tab => {
  tab.addEventListener('click', TabActivate);
});


var userVoice = 'SMOrc';
function LoadVoices() {
  ws.send(JSON.stringify({ call: 'ListVoices' }));
}
function ListVoicesResponse(voices) {
  let html = '';
  for (let i in voices) {
    html += '<audio loop id="voice-' + i + '" src="voices/' + voices[i] + '.mp3"></audio>';
  }
  for (let i in voices) {
    let voice = voices[i];
    let voiceShort = voice.split('.')[0];
    html += '<button ';
    if (voiceShort == userVoice) {
      html += 'class="selected" ';
    }
    html += 'onclick="SetVoice(\'' + voice + '\')" onmouseenter="document.getElementById(\'voice-' + i + '\').play()" onmouseleave="document.getElementById(\'voice-' + i + '\').pause()">' + voiceShort + '</button>';
  }
  document.getElementById('voices').innerHTML = html;
}
function SetVoice(voice) {
  userVoice = voice.split('.')[0];
  ws.send(JSON.stringify({ call: 'SetVoice', args: [voice] }));
  LoadVoices();
}

const chat = document.getElementById('chat');
var password = localStorage.getItem('password');
if (!password) {
  // Generate random password - base64 of random 18 bytes
  // 16 bytes should be enough (128 bits of security) but since
  // we're encoding in base64 we might as well round this to 18
  password = "";
  let arr = new Uint8Array(18);
  window.crypto.getRandomValues(arr);
  for (let i = 0; i < arr.length; ++i) {
    password += String.fromCharCode(arr[i]);
  }
  password = btoa(password);
  localStorage.setItem('password', password);
}
var ws;
function OnOpen() {
  chat.textContent = '';
  ws.send(JSON.stringify({ call: 'Password', args: [password] }));
}
function Reload() {
  chat.textContent = 'Reloading...';
  window.location.reload();
}
function CopyLoginCommand() {
  let loginCommand = document.getElementById('login-command');
  loginCommand.select();
  loginCommand.setSelectionRange(0, 99999);
  document.execCommand('copy');
  ShowAlert('<span style="font-family: Belanosima">Copied to clipboard</span>', 500);
}
function Welcome(user) {
  console.log('Welcome', user);
  let loginCommand = document.getElementById('login-command');
  if (!loginCommand) {
    return;
  }
  loginCommand.value = '!login ' + user.ticket;
  if (user.twitch) {
    document.getElementById('twitch-link').innerHTML = '<a href="https://twitch.tv/' + user.twitch.login + '" target="_blank">' + user.twitch.name + '</a>';
  } else {
    document.getElementById('twitch-link').innerHTML = 'Not linked';
  }
  if (user.youtube) {
    document.getElementById('youtube-link').innerHTML = '<a href="https://youtube.com/channel/' + user.youtube.channel + '" target="_blank"><img class="avatar" src="' + user.youtube.avatar_url + '">' + user.youtube.name + '</a>';
  } else {
    document.getElementById('youtube-link').innerHTML = 'Not linked';
  }
  let voicesSpan = document.getElementById('voices');
  if (user.voice) {
    userVoice = user.voice.split('.')[0];
  }
  voicesSpan.innerHTML = '<button class="selected" onclick="LoadVoices()">' + userVoice + '</button>';
}
let ecg_pings = {
  'Twitch': [],
  'YouTube': [],
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
  while (ecg_pings[component].length > 30) {
    ecg_pings[component].shift();
  }
}
function DrawECG(t) {
  for (let component in ecg_pings) {
    let canvas = document.getElementById('ecg-' + component);
    if (!canvas) {
      continue;
    }
    let W = canvas.clientWidth;
    let H = canvas.clientHeight;
    canvas.width = W;
    canvas.height = H;
    let ctx = canvas.getContext('2d');
    let now = Date.now();
    ctx.clearRect(0, 0, canvas.width, canvas.height);
    let y = H * 0.85;
    let X = 0;
    ctx.strokeStyle = '#ffffff';
    ctx.font = '40px Audiowide';
    ctx.lineJoin = 'round';
    ctx.miterLimit = 100;

    let pings = ecg_pings[component];
    ctx.beginPath();
    let h_good = H * 0.15;
    let h_bad = y;
    let h = h_bad;
    let widthTime = 6000;
    for (let i = 0; i < pings.length; ++i) {
      let entry = pings[i];
      let x = (now - entry.time) / widthTime * W;
      if (x <= W) {
        break;
      }
      if (entry.type == 'ping') {
        h = h_bad;
      } else if (entry.type == 'pong') {
        h = h_good;
      }
    }
    let line = new Path2D();
    line.moveTo(W, h);
    for (let i = 0; i < pings.length; ++i) {
      let entry = pings[i];
      let time = entry.time;
      let x = (now - time) / widthTime * W;
      if (x > W) {
        continue;
      }
      if (entry.type == 'ping') {
        line.lineTo(x, h);
        line.lineTo(x, h_bad);
        h = h_bad;
      } else if (entry.type == 'pong') {
        line.lineTo(x, h_good - 4);
        line.lineTo(x, h_good);
        h = h_good;
      }
    }
    line.lineTo(0, h);

    let good_shape = new Path2D(line);
    good_shape.lineTo(0, h_bad);
    good_shape.lineTo(W, h_bad);
    good_shape.closePath();
    let good_gradient = ctx.createLinearGradient(0, h_good, 0, h_bad);
    good_gradient.addColorStop(0, 'rgba(0, 255, 0, 0.5)');
    good_gradient.addColorStop(1, 'rgba(0, 255, 0, 0.0)');
    ctx.fillStyle = good_gradient;
    ctx.fill(good_shape);

    let bad_shape = new Path2D(line);
    bad_shape.lineTo(0, h_good);
    bad_shape.lineTo(W, h_good);
    bad_shape.closePath();
    let bad_gradient = ctx.createLinearGradient(0, h_good, 0, h_bad);
    bad_gradient.addColorStop(1, 'rgba(255, 0, 0, 1.0)');
    bad_gradient.addColorStop(0, 'rgba(255, 0, 0, 0.0)');
    ctx.fillStyle = bad_gradient;
    ctx.fill(bad_shape);

    ctx.lineWidth = 4;
    ctx.filter = "blur(4px)";
    ctx.strokeStyle = '#ffffff';
    ctx.stroke(line);
    ctx.filter = "blur(0px)";
    ctx.lineWidth = 2;
    ctx.strokeStyle = '#ffffff';
    ctx.stroke(line);
    ctx.globalCompositeOperation = 'source-over';
    ctx.translate(W, 0);
    ctx.lineJoin = 'round';
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
function SetStreamTitle(title) {
  document.title = title;
  let titleElement = document.getElementById('title');
  if (titleElement) {
    titleElement.value = title;
  }
}
function Connect() {
  let protocol = location.protocol == 'https:' ? 'wss:' : 'ws:';
  let domain = (location.host == "" || location.host == "absolute") ? 'localhost:3447' : location.host;
  ws = new WebSocket(protocol + '//' + domain + '/live/ws');
  ws.onopen = OnOpen;
  ws.onmessage = OnMessage;
  ws.onclose = OnClose;
}
function OnClose() {
  for (let component of Object.keys(ecg_pings)) {
    Ping(component);
  }
  chat.textContent = 'Connection lost. Reconnecting...';
  setTimeout(Connect, 1000);
}
Connect();
