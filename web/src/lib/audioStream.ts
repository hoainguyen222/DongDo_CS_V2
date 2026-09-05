// ============================================================
// audioStream — Lightweight audio utilities (NO WebRTC)
// ============================================================
// Purpose: provide minimal browser-audio primitives used in the
// click-to-call flow:
//   - requestMicrophonePermission(): used to "test mic" UX before
//     a guest clicks "Yêu cầu gọi lại". Media is acquired and
//     immediately released so we never keep a mic stream alive on
//     the frontend (Asterisk + SIP client handle the real audio).
//   - playNotificationTone(): short ringing/notification tone played
//     on the admin side when an incoming call_ring WS event fires.
//     Uses WebAudio so we don't need an asset file.
// ============================================================

/**
 * Request microphone permission from the browser.
 * Returns a boolean indicating whether the user granted permission.
 * The acquired MediaStream is stopped immediately after the test
 * (so we don't keep the mic open in the background).
 */
export async function requestMicrophonePermission(): Promise<boolean> {
  if (typeof window === 'undefined' || !navigator?.mediaDevices?.getUserMedia) {
    return false;
  }
  try {
    const stream = await navigator.mediaDevices.getUserMedia({
      audio: {
        echoCancellation: true,
        noiseSuppression: true,
        autoGainControl: true,
      },
      video: false,
    });
    // Immediately release the test stream — Asterisk will handle real audio.
    stream.getTracks().forEach((track) => {
      try {
        track.stop();
      } catch (_) {
        /* noop */
      }
    });
    return true;
  } catch (_err) {
    return false;
  }
}

/**
 * Play a short notification tone on the device. Used by the admin
 * UI when an incoming call_ring event arrives so the agent knows
 * to pick up the SIP client / softphone.
 */
export async function playNotificationTone(): Promise<void> {
  if (typeof window === 'undefined') return;
  try {
    const AudioCtx = window.AudioContext || (window as any).webkitAudioContext;
    if (!AudioCtx) return;
    const ctx = new AudioCtx();
    if (ctx.state === 'suspended') {
      try {
        await ctx.resume();
      } catch (_) {
        /* noop */
      }
    }

    // Simple two-tone ring (440Hz → 660Hz) ~600ms total
    const now = ctx.currentTime;
    const tones: Array<[number, number, number]> = [
      [now + 0.0, 0.18, 880],
      [now + 0.25, 0.18, 660],
      [now + 0.55, 0.18, 880],
    ];
    for (const [start, duration, freq] of tones) {
      const osc = ctx.createOscillator();
      const gain = ctx.createGain();
      osc.type = 'sine';
      osc.frequency.value = freq;
      gain.gain.setValueAtTime(0, start);
      gain.gain.linearRampToValueAtTime(0.25, start + 0.02);
      gain.gain.linearRampToValueAtTime(0, start + duration);
      osc.connect(gain).connect(ctx.destination);
      osc.start(start);
      osc.stop(start + duration);
    }

    // Close the AudioContext once the last tone ends so we don't leak resources.
    const lastEnd = tones[tones.length - 1][0] + tones[tones.length - 1][1] + 0.1;
    setTimeout(() => {
      try {
        ctx.close();
      } catch (_) {
        /* noop */
      }
    }, Math.max(0, (lastEnd - ctx.currentTime) * 1000));
  } catch (err) {
    // Fail silently — notification tone is purely UX.
    // eslint-disable-next-line no-console
    console.warn('[audioStream] playNotificationTone failed:', err);
  }
}

/**
 * Build a click-to-call SIP URI. When the agent clicks "Mở softphone"
 * we launch the user's default SIP handler (Linphone, Zoiper, etc.)
 * with the destination guest phone number.
 */
export function buildSipUri(target: string, asteriskHost: string = 'dongdo.local'): string {
  const safeTarget = encodeURIComponent(target || '');
  return `sip:guest@${asteriskHost}?target=${safeTarget}`;
}
