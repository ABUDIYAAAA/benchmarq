'use strict';

// Holds the first camera's stream and returns a JPEG frame whenever the main
// process asks for one (window.capture is provided by capture-preload.js).

const WIDTH = 640;
const HEIGHT = 480;
const JPEG_QUALITY = 0.2;

const video = document.getElementById('video');
const canvas = document.getElementById('canvas');
canvas.width = WIDTH;
canvas.height = HEIGHT;

let stream = null;

function live() {
  return stream && stream.getVideoTracks().some((t) => t.readyState === 'live');
}

/** Opens the first camera the system lists. */
async function open() {
  stream?.getTracks().forEach((t) => t.stop());
  // A first request is needed before device IDs and labels are available.
  const probe = await navigator.mediaDevices.getUserMedia({ video: true, audio: false });
  probe.getTracks().forEach((t) => t.stop());

  const cameras = (await navigator.mediaDevices.enumerateDevices()).filter((d) => d.kind === 'videoinput');
  if (cameras.length === 0) throw new Error('no camera found');

  stream = await navigator.mediaDevices.getUserMedia({
    audio: false,
    video: { deviceId: { exact: cameras[0].deviceId }, width: { ideal: WIDTH }, height: { ideal: HEIGHT } },
  });
  video.srcObject = stream;
  await video.play();
  // Give auto-exposure a moment so the first frame is not black.
  await new Promise((resolve) => setTimeout(resolve, 500));
}

async function frame() {
  if (!live()) await open();
  canvas.getContext('2d').drawImage(video, 0, 0, WIDTH, HEIGHT);
  const blob = await new Promise((resolve) => canvas.toBlob(resolve, 'image/jpeg', JPEG_QUALITY));
  if (!blob) throw new Error('could not encode frame');
  return blob.arrayBuffer();
}

window.capture.onTake(async (id) => {
  try {
    window.capture.reply(id, await frame(), null);
  } catch (err) {
    window.capture.reply(id, null, String(err?.message ?? err));
  }
});

open().catch(() => {}); // warm up; errors are reported on the first capture
