// Command replay_visual renders a finished run's transcript as a self-contained
// HTML animation. It reads the same structured turn records as scripts/replay,
// through the same harness.LoadTranscript parser; the engine stays headless.
//
//	go run ./scripts/replay_visual [-out replay.html] transcripts/run_….jsonl
package main

import (
	"flag"
	"fmt"
	"html/template"
	"io"
	"os"
	"path/filepath"
	"strings"

	"architect_mockup_project/harness"
	"architect_mockup_project/internal/visual"
)

func main() {
	out := flag.String("out", "", "output HTML path (default <transcript>.html)")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: replay_visual [-out replay.html] <transcript.jsonl>")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}

	in := flag.Arg(0)
	f, err := os.Open(in)
	if err != nil {
		fail("%v", err)
	}
	defer f.Close()

	log, loadErr := harness.LoadTranscript(f)
	sc := visual.BuildScene(log)

	dest := *out
	if dest == "" {
		dest = strings.TrimSuffix(in, filepath.Ext(in)) + ".html"
	}
	of, err := os.Create(dest)
	if err != nil {
		fail("%v", err)
	}
	defer of.Close()
	if err := writeHTML(of, sc); err != nil {
		fail("%v", err)
	}
	if loadErr != nil {
		fmt.Fprintf(os.Stderr, "replay_visual: %v\n", loadErr)
	}
	fmt.Println(dest)
}

func writeHTML(w io.Writer, sc visual.Scene) error {
	return pageTemplate.Execute(w, sc)
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

// pageTemplate is a single self-contained file: no assets, no network, no
// framework. html/template escapes the scene into the script as valid JS/JSON,
// so reasoning text containing "</script>" cannot break out.
var pageTemplate = template.Must(template.New("page").Parse(pageHTML))

const pageHTML = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>replay · {{.Model}}</title>
<style>
  :root { color-scheme: dark; }
  body { margin: 0; background: #0d0f16; color: #e6e8ef;
         font: 14px ui-monospace, SFMono-Regular, Menlo, monospace; }
  #stage { display: block; margin: 16px auto 0; background: #12141c;
           border: 1px solid #232635; border-radius: 8px; }
  #controls { display: flex; gap: 12px; align-items: center; justify-content: center;
              margin: 12px; }
  button { background: #232635; color: #e6e8ef; border: 1px solid #2c3145;
           border-radius: 6px; padding: 6px 12px; font: inherit; cursor: pointer; }
  button:hover { background: #2c3145; }
  label { color: #8b90a0; }
</style>
</head>
<body>
<canvas id="stage" width="960" height="560"></canvas>
<div id="controls">
  <button id="play">play</button>
  <button id="prev">prev</button>
  <button id="next">next</button>
  <button id="restart">restart</button>
  <label>speed <input id="speed" type="range" min="150" max="1500" step="50" value="700"></label>
</div>
<script>
const SCENE = {{.}};
const cvs = document.getElementById('stage');
const ctx = cvs.getContext('2d');
const F = SCENE.frames || [];
let idx = 0, playing = false, speed = 700, timer = null;

const maxBal = Math.max.apply(null, [1].concat(F.map(function (f) { return f.balance; })));

function pad(n) { return n.toLocaleString(); }

function wrap(text, x, y, maxW, lineH, maxLines) {
  const words = String(text).split(/\s+/);
  let line = '', lines = 0;
  for (let i = 0; i < words.length; i++) {
    const test = line ? line + ' ' + words[i] : words[i];
    if (ctx.measureText(test).width > maxW && line) {
      ctx.fillText(line, x, y + lines * lineH);
      lines++;
      if (lines >= maxLines) { ctx.fillText('\u2026', x, y + lines * lineH); return; }
      line = words[i];
    } else {
      line = test;
    }
  }
  if (line) ctx.fillText(line, x, y + lines * lineH);
}

function draw() {
  const W = cvs.width;
  ctx.fillStyle = '#12141c';
  ctx.fillRect(0, 0, W, cvs.height);
  ctx.textBaseline = 'alphabetic';
  ctx.textAlign = 'left';

  ctx.fillStyle = '#e6e8ef';
  ctx.font = 'bold 20px ui-monospace, monospace';
  ctx.fillText(SCENE.model + '  \u00b7  ' + SCENE.provider, 40, 44);
  ctx.fillStyle = '#8b90a0';
  ctx.font = '13px ui-monospace, monospace';
  ctx.fillText('engine seed ' + SCENE.engine_seed + '   \u00b7   final ' + pad(SCENE.final_balance) +
               ' krones   \u00b7   earned ' + pad(SCENE.total_earned), 40, 66);
  if (SCENE.diverged_at) {
    ctx.fillStyle = '#f2b04a';
    ctx.fillText('! engine replay diverged from the recorded run at turn ' + SCENE.diverged_at, 40, 86);
  }

  if (!F.length) {
    ctx.fillStyle = '#8b90a0';
    ctx.font = '16px ui-monospace, monospace';
    ctx.fillText('no turn records in this transcript', 40, 140);
    return;
  }

  const f = F[idx];

  const tx = 60, tw = W - 120, ty = 130;
  ctx.fillStyle = '#8b90a0';
  ctx.font = '13px ui-monospace, monospace';
  ctx.fillText('health   ' + Math.max(0, SCENE.max_day - f.day) + ' / ' + SCENE.max_day + '   (day ' + f.day + ')', tx, ty - 14);
  ctx.fillStyle = '#232635';
  ctx.fillRect(tx, ty - 6, tw, 12);
  const prog = Math.max(0, 1 - f.day / SCENE.max_day); // health drains as days pass
  ctx.fillStyle = '#3b6ea5';
  ctx.fillRect(tx, ty - 6, tw * prog, 12);
  const px = tx + tw * prog;
  ctx.fillStyle = '#ffd166';
  ctx.fillRect(px - 7, ty - 15, 14, 30);
  ctx.strokeStyle = '#12141c';
  ctx.strokeRect(px - 7, ty - 15, 14, 30);

  const by = 220, bw = W - 120;
  ctx.fillStyle = '#8b90a0';
  ctx.font = '13px ui-monospace, monospace';
  ctx.fillText('balance   ' + pad(f.balance) + ' krones', tx, by - 12);
  ctx.fillStyle = '#232635';
  ctx.fillRect(tx, by, bw, 18);
  const bp = maxBal ? f.balance / maxBal : 0;
  ctx.fillStyle = f.error ? '#e05c5c' : '#5bd67e';
  ctx.fillRect(tx, by, bw * bp, 18);

  const cy = 290, cw = W - 120, ch = 120;
  ctx.fillStyle = '#1a1d28';
  ctx.fillRect(tx, cy, cw, ch);
  ctx.strokeStyle = f.error ? '#e05c5c' : '#2c3145';
  ctx.lineWidth = 2;
  ctx.strokeRect(tx, cy, cw, ch);
  ctx.lineWidth = 1;

  ctx.fillStyle = '#e6e8ef';
  ctx.font = 'bold 18px ui-monospace, monospace';
  ctx.fillText(f.action + (f.quest_id ? ' ' + f.quest_id : ''), tx + 16, cy + 34);

  ctx.font = '13px ui-monospace, monospace';
  if (f.error) {
    ctx.fillStyle = '#e05c5c';
    ctx.fillText('invalid: ' + f.error, tx + 16, cy + 58);
  } else {
    ctx.fillStyle = '#5bd67e';
    ctx.fillText('+' + pad(f.earned) + ' krones   \u00b7   ' + f.days_spent +
                 ' day' + (f.days_spent === 1 ? '' : 's'), tx + 16, cy + 58);
  }
  ctx.fillStyle = '#9aa2b8';
  ctx.font = '13px ui-monospace, monospace';
  wrap(f.reasoning || '(no reasoning recorded)', tx + 16, cy + 84, cw - 32, 18, 2);

  const ly = 450;
  ctx.fillStyle = '#8b90a0';
  ctx.font = '13px ui-monospace, monospace';
  ctx.fillText('turn log', tx, ly - 10);
  const start = Math.max(0, idx - 4);
  for (let i = start, r = 0; i <= idx; i++, r++) {
    const t = F[i];
    ctx.fillStyle = t.error ? '#e05c5c' : '#9aa2b8';
    ctx.fillText('day ' + t.day + '   bal ' + pad(t.balance) + '   ' + t.action +
                 (t.quest_id ? ' ' + t.quest_id : '') +
                 (t.error ? '   invalid' : '   +' + pad(t.earned)), tx, ly + 12 + r * 18);
  }
}

function setIdx(i) {
  idx = Math.max(0, Math.min(F.length - 1, i));
  draw();
}
function stop() {
  playing = false;
  if (timer) { clearInterval(timer); timer = null; }
  document.getElementById('play').textContent = 'play';
}
function tick() {
  if (idx >= F.length - 1) { stop(); return; }
  idx++;
  draw();
}
function play() {
  if (!F.length) return;
  if (idx >= F.length - 1) idx = 0;
  playing = true;
  document.getElementById('play').textContent = 'pause';
  if (timer) clearInterval(timer);
  timer = setInterval(tick, speed);
}
document.getElementById('play').addEventListener('click', function () {
  if (playing) { stop(); } else { play(); }
});
document.getElementById('prev').addEventListener('click', function () { stop(); setIdx(idx - 1); });
document.getElementById('next').addEventListener('click', function () { stop(); setIdx(idx + 1); });
document.getElementById('restart').addEventListener('click', function () { stop(); setIdx(0); });
document.getElementById('speed').addEventListener('input', function (e) {
  speed = Number(e.target.value);
  if (playing) { clearInterval(timer); timer = setInterval(tick, speed); }
});
draw();
</script>
</body>
</html>
`
