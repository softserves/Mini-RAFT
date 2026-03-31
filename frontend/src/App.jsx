import { useEffect, useRef, useState } from "react";

const GATEWAY_URL = "ws://localhost:8080/ws";

function drawSegment(context, from, to, color, width) {
  if (!from || !to) return;
  context.strokeStyle = color;
  context.lineWidth = width;
  context.lineCap = "round";
  context.lineJoin = "round";
  context.beginPath();
  context.moveTo(from.x, from.y);
  context.lineTo(to.x, to.y);
  context.stroke();
}

function renderStroke(context, stroke) {
  if (!stroke?.points || stroke.points.length < 2) return;
  for (let i = 1; i < stroke.points.length; i++) {
    drawSegment(
      context,
      stroke.points[i - 1],
      stroke.points[i],
      stroke.color,
      stroke.width
    );
  }
}

export default function App() {
  const canvasRef = useRef(null);
  const toolbarRef = useRef(null);
  const wsRef = useRef(null);
  const reconnectRef = useRef(null);
  const isDrawingRef = useRef(false);
  const currentStrokeRef = useRef([]);
  const strokeLogRef = useRef([]);

  const [color, setColor] = useState("#000000");
  const [brushSize, setBrushSize] = useState(4);
  const [status, setStatus] = useState("Connecting...");
  const [statusClass, setStatusClass] = useState("disconnected");
  const [showDebug, setShowDebug] = useState(false);
  const [booting, setBooting] = useState(true);
  const [debug, setDebug] = useState({
    clientsConnected: 0,
    currentLeaderUrl: "unknown",
    replicas: [],
    replicaStatus: [],
    aliveReplicas: 0,
    timestamp: null,
  });

  const replayAll = () => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const ctx = canvas.getContext("2d");
    ctx.clearRect(0, 0, canvas.width, canvas.height);
    strokeLogRef.current.forEach((stroke) => renderStroke(ctx, stroke));
  };

  const resizeCanvas = () => {
    const canvas = canvasRef.current;
    const toolbar = toolbarRef.current;
    if (!canvas || !toolbar) return;

    canvas.width = window.innerWidth;
    canvas.height = window.innerHeight - toolbar.offsetHeight;
    replayAll();
  };

  const send = (data) => {
    if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
      wsRef.current.send(JSON.stringify(data));
    }
  };

  const connect = () => {
    wsRef.current = new WebSocket(GATEWAY_URL);

    wsRef.current.onopen = () => {
      setStatus("Connected");
      setStatusClass("connected");
      setBooting(false);
      if (reconnectRef.current) {
        clearTimeout(reconnectRef.current);
      }
    };

    wsRef.current.onmessage = (event) => {
      let msg;
      try {
        msg = JSON.parse(event.data);
      } catch {
        return;
      }

      const canvas = canvasRef.current;
      if (!canvas) return;
      const ctx = canvas.getContext("2d");

      if (msg.type === "stroke") {
        strokeLogRef.current.push(msg);
        renderStroke(ctx, msg);
      } else if (msg.type === "clear") {
        strokeLogRef.current = [];
        ctx.clearRect(0, 0, canvas.width, canvas.height);
      } else if (msg.type === "init") {
        strokeLogRef.current = msg.strokes || [];
        replayAll();
      }
    };

    wsRef.current.onclose = () => {
      setStatus("Disconnected, reconnecting...");
      setStatusClass("disconnected");
      reconnectRef.current = setTimeout(connect, 2000);
    };

    wsRef.current.onerror = () => {
      wsRef.current?.close();
    };
  };

  useEffect(() => {
    connect();
    resizeCanvas();
    window.addEventListener("resize", resizeCanvas);

    return () => {
      window.removeEventListener("resize", resizeCanvas);
      if (reconnectRef.current) {
        clearTimeout(reconnectRef.current);
      }
      wsRef.current?.close();
    };
  }, []);

  useEffect(() => {
    const pullDebugStats = async () => {
      try {
        const resp = await fetch("http://localhost:8080/debug/stats");
        if (!resp.ok) return;
        const data = await resp.json();
        setDebug(data);
      } catch {
        // keep last-known debug snapshot
      }
    };

    pullDebugStats();
    const timer = setInterval(pullDebugStats, 1500);
    return () => clearInterval(timer);
  }, []);

  useEffect(() => {
    resizeCanvas();
  }, [showDebug]);

  const onPointerDown = (e) => {
    isDrawingRef.current = true;
    currentStrokeRef.current = [{ x: e.nativeEvent.offsetX, y: e.nativeEvent.offsetY }];
  };

  const onPointerMove = (e) => {
    if (!isDrawingRef.current) return;
    const canvas = canvasRef.current;
    if (!canvas) return;
    const ctx = canvas.getContext("2d");

    const point = { x: e.nativeEvent.offsetX, y: e.nativeEvent.offsetY };
    const current = currentStrokeRef.current;
    current.push(point);

    drawSegment(ctx, current[current.length - 2], point, color, brushSize);
  };

  const finishStroke = () => {
    if (!isDrawingRef.current) return;
    isDrawingRef.current = false;

    const current = currentStrokeRef.current;
    if (current.length < 2) {
      currentStrokeRef.current = [];
      return;
    }

    send({
      type: "stroke",
      points: current,
      color,
      width: Number(brushSize),
    });

    currentStrokeRef.current = [];
  };

  const clearCanvas = () => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const ctx = canvas.getContext("2d");

    strokeLogRef.current = [];
    ctx.clearRect(0, 0, canvas.width, canvas.height);
    send({ type: "clear" });
  };

  const strokeCount = strokeLogRef.current.length;

  return (
    <div className="app-shell">
      {booting && (
        <div className="loading-screen">
          <div className="loading-card">
            <div className="spinner" />
            <h2>Connecting to Mini-RAFT...</h2>
            <p>Preparing collaborative canvas and replica cluster status.</p>
          </div>
        </div>
      )}

      <header id="toolbar" ref={toolbarRef}>
        <div className="brand">Mini-RAFT Canvas</div>

        <label className="control">
          <span>Color</span>
          <input
            type="color"
            value={color}
            onChange={(e) => setColor(e.target.value)}
          />
        </label>

        <label className="control">
          <span>Size</span>
          <input
            type="range"
            min="1"
            max="20"
            value={brushSize}
            onChange={(e) => setBrushSize(Number(e.target.value))}
          />
          <strong>{brushSize}</strong>
        </label>

        <button className="clear-btn" onClick={clearCanvas}>Clear</button>

        <button
          className={`clear-btn debug-toggle ${showDebug ? "active" : ""}`}
          onClick={() => setShowDebug((v) => !v)}
        >
          {showDebug ? "Hide Debug" : "Show Debug"}
        </button>

        <span id="status" className={statusClass}>{status}</span>
      </header>

      {showDebug && (
        <>
          <section className="debug-strip">
            <div className="debug-chip">
              <strong>Clients</strong>
              <span>{debug.clientsConnected ?? 0}</span>
            </div>
            <div className="debug-chip">
              <strong>Leader</strong>
              <span>{debug.currentLeaderUrl || "unknown"}</span>
            </div>
            <div className="debug-chip">
              <strong>Strokes</strong>
              <span>{strokeCount}</span>
            </div>
            <div className="debug-chip replicas">
              <strong>Replicas</strong>
              <span>
                {debug.aliveReplicas ?? 0}/{Array.isArray(debug.replicas) ? debug.replicas.length : 0}
              </span>
            </div>
          </section>

          <section className="replica-status-row">
            {(debug.replicaStatus || []).map((r) => (
              <div key={r.url} className={`replica-pill ${r.alive ? "up" : "down"}`}>
                <span className="dot" />
                <span>{r.url.replace("http://", "")}</span>
              </div>
            ))}
          </section>
        </>
      )}

      {showDebug && (debug.replicaStatus || []).length === 0 && (
        <section className="draw-hint-row">
          <div className="hint-pill">Loading cluster debug info...</div>
        </section>
      )}

      <canvas
        ref={canvasRef}
        id="canvas"
        onMouseDown={onPointerDown}
        onMouseMove={onPointerMove}
        onMouseUp={finishStroke}
        onMouseLeave={finishStroke}
      />
    </div>
  );
}
