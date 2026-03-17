function makeSignalingUrl(token) {
  const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
  return `${protocol}//${window.location.host}/ws?token=${encodeURIComponent(token)}`;
}

function createPlaceholderStream() {
  const canvas = document.createElement("canvas");
  canvas.width = 1280;
  canvas.height = 720;
  const context = canvas.getContext("2d");

  let tick = 0;
  const paint = () => {
    tick += 1;
    context.fillStyle = "#020617";
    context.fillRect(0, 0, canvas.width, canvas.height);
    context.fillStyle = "#38bdf8";
    context.font = "42px sans-serif";
    context.fillText("Ожидание видеопотока от хоста", 80, 160);
    context.fillStyle = "#94a3b8";
    context.font = "28px sans-serif";
    context.fillText(`Сигналинг активен: ${new Date().toLocaleTimeString()}`, 80, 220);
    context.fillStyle = tick % 120 < 60 ? "#22c55e" : "#f59e0b";
    context.beginPath();
    context.arc(104, 104, 18, 0, Math.PI * 2);
    context.fill();
  };

  paint();
  window.setInterval(paint, 1000);
  return canvas.captureStream(5);
}

export async function createRemoteConnection({ token, videoElement, onStatus, onInputMessage }) {
  const signaling = new WebSocket(makeSignalingUrl(token));
  const peer = new RTCPeerConnection();
  let placeholderActivated = false;
  const pendingControlMessages = [];

  const attachPlaceholder = () => {
    if (placeholderActivated || videoElement.srcObject) {
      return;
    }

    placeholderActivated = true;
    videoElement.srcObject = createPlaceholderStream();
    onStatus("Сигналинг активен, ожидается медиатрек");
  };

  const flushPendingControls = () => {
    while (pendingControlMessages.length > 0) {
      const serialized = pendingControlMessages.shift();
      if (inputChannel.readyState === "open") {
        inputChannel.send(serialized);
        continue;
      }

      if (signaling.readyState === WebSocket.OPEN) {
        signaling.send(serialized);
        continue;
      }

      pendingControlMessages.unshift(serialized);
      return;
    }
  };

  const inputChannel = peer.createDataChannel("control");
  inputChannel.addEventListener("open", flushPendingControls);
  inputChannel.addEventListener("message", (event) => {
    try {
      onInputMessage?.(JSON.parse(event.data));
    } catch {
      onInputMessage?.({ type: "raw", payload: event.data });
    }
  });

  peer.addTransceiver("video", { direction: "recvonly" });
  peer.ontrack = (event) => {
    const [stream] = event.streams;
    if (stream) {
      videoElement.srcObject = stream;
      onStatus("Видео подключено");
    }
  };

  peer.onicecandidate = (event) => {
    if (!event.candidate || signaling.readyState !== WebSocket.OPEN) {
      return;
    }

    signaling.send(
      JSON.stringify({
        type: "webrtc.ice",
        candidate: event.candidate,
      }),
    );
  };

  signaling.addEventListener("open", async () => {
    onStatus("WebSocket подключен");
    flushPendingControls();
    const offer = await peer.createOffer();
    await peer.setLocalDescription(offer);
    signaling.send(
      JSON.stringify({
        type: "webrtc.offer",
        sdp: offer.sdp,
      }),
    );
    window.setTimeout(attachPlaceholder, 3000);
  });

  signaling.addEventListener("message", async (event) => {
    const message = JSON.parse(event.data);
    switch (message.type) {
      case "webrtc.answer":
        await peer.setRemoteDescription({
          type: "answer",
          sdp: message.sdp,
        });
        onStatus("WebRTC согласован");
        break;
      case "webrtc.ice":
        await peer.addIceCandidate(message.candidate);
        break;
      case "error":
        throw new Error(message.message);
      default:
        break;
    }
  });

  signaling.addEventListener("close", () => {
    onStatus("Соединение закрыто");
    attachPlaceholder();
  });

  return {
    signaling,
    peer,
    inputChannel,
    sendControl(payload) {
      const serialized = JSON.stringify(payload);
      if (inputChannel.readyState === "open") {
        inputChannel.send(serialized);
        return;
      }

      if (signaling.readyState === WebSocket.OPEN) {
        signaling.send(serialized);
        return;
      }

      pendingControlMessages.push(serialized);
    },
  };
}
