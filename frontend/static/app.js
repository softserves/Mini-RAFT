
const GATEWAY_URL = "ws://localhost:8080/ws";

//Canvas Setup
const canvas = document.getElementById("canvas");
const ctx = canvas.getContext("2d");

function resizeCanvas() {

  const imageData = ctx.getImageData(0, 0, canvas.width, canvas.height);
  canvas.width = window.innerWidth;
  canvas.height = window.innerHeight - document.getElementById("toolbar").offsetHeight;
  ctx.putImageData(imageData, 0, 0);
}
window.addEventListener("resize", resizeCanvas);
resizeCanvas();

//state
let isDrawing = false;
let currentStroke = [];       
let strokeLog = [];            

//toolbar
const colorPicker = document.getElementById("colorPicker");
const brushSize   = document.getElementById("brushSize");
const statusEl    = document.getElementById("status");

document.getElementById("clearBtn").addEventListener("click", () => {

  send({ type: "clear" });
});

//Drawing
canvas.addEventListener("mousedown", (e) => {
  isDrawing = true;
  currentStroke = [{ x: e.offsetX, y: e.offsetY }];
  ctx.beginPath();
  ctx.moveTo(e.offsetX, e.offsetY);
});

canvas.addEventListener("mousemove", (e) => {
  if (!isDrawing) return;
  const point = { x: e.offsetX, y: e.offsetY };
  currentStroke.push(point);
  // Draw locally for immediate feedback; full stroke will be sent on mouseup
  drawSegment(ctx, currentStroke[currentStroke.length - 2], point, colorPicker.value, brushSize.value);
});

canvas.addEventListener("mouseup", () => {
  if (!isDrawing) return;
  isDrawing = false;
  if (currentStroke.length < 2) return;

  // serialize and sending the stroke to gateway
  const stroke = {
    type:   "stroke",
    points: currentStroke,
    color:  colorPicker.value,
    width:  parseInt(brushSize.value),
  };
  send(stroke);
  currentStroke = [];
});

canvas.addEventListener("mouseleave", () => {
  if (isDrawing) {
    isDrawing = false;
    currentStroke = [];
  }
});

// rendering
function drawSegment(context, from, to, color, width) {
  if (!from || !to) return;
  context.strokeStyle = color;
  context.lineWidth   = width;
  context.lineCap     = "round";
  context.lineJoin    = "round";
  context.beginPath();
  context.moveTo(from.x, from.y);
  context.lineTo(to.x, to.y);
  context.stroke();
}

function renderStroke(stroke) {
  if (!stroke.points || stroke.points.length < 2) return;
  for (let i = 1; i < stroke.points.length; i++) {
    drawSegment(ctx, stroke.points[i - 1], stroke.points[i], stroke.color, stroke.width);
  }
}

function replayAll() {
  ctx.clearRect(0, 0, canvas.width, canvas.height);
  strokeLog.forEach(renderStroke);
}

// webSocket
let ws = null;
let reconnectTimer = null;

function connect() {
  ws = new WebSocket(GATEWAY_URL);

  ws.onopen = () => {
    setStatus("Connected", "connected");
    clearTimeout(reconnectTimer);
  };

  ws.onmessage = (event) => {
    let msg;
    try { msg = JSON.parse(event.data); }
    catch { return; }

    if (msg.type === "stroke") {
      strokeLog.push(msg);
      renderStroke(msg);
    } else if (msg.type === "clear") {
      strokeLog = [];
      ctx.clearRect(0, 0, canvas.width, canvas.height);
    } else if (msg.type === "init") {
      //log replay on first connect / reconnect
      strokeLog = msg.strokes || [];
      replayAll();
    }
  };

  ws.onclose = () => {
    setStatus("Disconnected — reconnecting", "disconnected");
    reconnectTimer = setTimeout(connect, 2000);
  };

  ws.onerror = () => {
    ws.close();
  };
}

function send(data) {
  if (ws && ws.readyState === WebSocket.OPEN) {
    ws.send(JSON.stringify(data));
  }
}

function setStatus(text, cls) {
  statusEl.textContent = text;
  statusEl.className = cls || "";
}

connect();
