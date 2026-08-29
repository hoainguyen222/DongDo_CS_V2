import { WSClient } from './ws';

const STUN_SERVERS: RTCConfiguration = {
  iceServers: [
    { urls: 'stun:stun.l.google.com:19302' },
    { urls: 'stun:stun1.l.google.com:19302' },
    { urls: 'stun:stun2.l.google.com:19302' },
  ],
};

export class WebRTCVoiceManager {
  private pc: RTCPeerConnection | null = null;
  private localStream: MediaStream | null = null;
  private remoteStream: MediaStream | null = null;
  private remoteAudio: HTMLAudioElement | null = null;
  private ws: WSClient;
  private sessionID: string;
  private callID: number | null = null;

  // Web Audio Mixer & Recorder
  private audioContext: AudioContext | null = null;
  private mixedDestination: MediaStreamAudioDestinationNode | null = null;
  private mediaRecorder: MediaRecorder | null = null;
  private recordedChunks: Blob[] = [];
  private recordingStopResolver: (() => void) | null = null;
  private queuedCandidates: RTCIceCandidate[] = [];

  private onCallStateChange?: (state: 'idle' | 'calling' | 'connected' | 'ended') => void;
  private onRemoteStream?: (stream: MediaStream) => void;

  constructor(
    ws: WSClient,
    sessionID: string,
    onStateOrStream?: ((state: 'idle' | 'calling' | 'connected' | 'ended') => void) | ((stream: MediaStream) => void),
    onStreamCallback?: (stream: MediaStream) => void
  ) {
    this.ws = ws;
    this.sessionID = sessionID;

    if (typeof onStreamCallback === 'function') {
      this.onCallStateChange = onStateOrStream as any;
      this.onRemoteStream = onStreamCallback;
    } else if (typeof onStateOrStream === 'function') {
      this.onRemoteStream = (stream) => {
        if (stream instanceof MediaStream) {
          try { (onStateOrStream as any)(stream); } catch (e) {}
        }
      };
      this.onCallStateChange = (state) => {
        if (typeof state === 'string') {
          try { (onStateOrStream as any)(state); } catch (e) {}
        }
      };
    }

    this.initRemoteAudio();
    this.setupSignaling();
  }

  public setCallID(id: number): void {
    this.callID = id;
  }

  private initRemoteAudio(): void {
    if (typeof window !== 'undefined') {
      this.remoteAudio = new Audio();
      this.remoteAudio.autoplay = true;
    }
  }

  private setupSignaling(): void {
    this.ws.on('call_offer', async (event) => {
      if (event.payload?.type === 'offer' || event.payload?.sdp) {
        await this.handleOffer(event.payload);
      }
    });

    this.ws.on('call_answer', async (event) => {
      if (event.payload?.type === 'answer' || event.payload?.sdp) {
        await this.handleAnswer(event.payload);
      }
    });

    this.ws.on('call_ice', async (event) => {
      if (event.payload?.candidate) {
        await this.handleCandidate(event.payload);
      }
    });

    this.ws.on('call_end', () => {
      this.onCallStateChange?.('ended');
      this.endCall(false);
    });
  }

  private async createPeerConnection(): Promise<RTCPeerConnection> {
    if (this.pc) {
      try { this.pc.close(); } catch (e) {}
    }

    this.pc = new RTCPeerConnection(STUN_SERVERS);
    this.queuedCandidates = [];

    // ICE Candidate handler
    this.pc.onicecandidate = (event) => {
      if (event.candidate) {
        this.ws.send('call_ice', '', event.candidate, this.sessionID);
      }
    };

    // Track handler (receive remote audio)
    this.pc.ontrack = (event) => {
      const incomingStream = event.streams[0];
      if (incomingStream) {
        this.remoteStream = incomingStream;
        if (this.remoteAudio) {
          this.remoteAudio.srcObject = incomingStream;
          this.remoteAudio.play().catch(() => {});
        }
        if (this.onRemoteStream) {
          this.onRemoteStream(incomingStream);
        }
        // Update mixer with remote audio
        this.setupAudioMixer();
      }
    };

    this.pc.onconnectionstatechange = () => {
      if (this.pc?.connectionState === 'connected') {
        this.onCallStateChange?.('connected');
        this.setupAudioMixer();
        this.startRecording();
      } else if (this.pc?.connectionState === 'disconnected' || this.pc?.connectionState === 'failed') {
        this.endCall(true);
      }
    };

    // Get microphone stream
    try {
      this.localStream = await navigator.mediaDevices.getUserMedia({
        audio: {
          echoCancellation: true,
          noiseSuppression: true,
          autoGainControl: true,
        },
        video: false,
      });
      this.localStream.getTracks().forEach((track) => {
        this.pc?.addTrack(track, this.localStream!);
      });
      this.setupAudioMixer();
      this.startRecording();
    } catch (err) {
      console.warn('Microphone permission denied or not available:', err);
    }

    return this.pc;
  }

  public async startCall(): Promise<void> {
    this.onCallStateChange?.('calling');
    const pc = await this.createPeerConnection();
    const offer = await pc.createOffer();
    await pc.setLocalDescription(offer);
    this.ws.send('call_offer', '', offer, this.sessionID);
    this.setupAudioMixer();
    this.startRecording();
  }

  public async acceptCall(incomingOffer?: RTCSessionDescriptionInit): Promise<void> {
    if (incomingOffer) {
      await this.handleOffer(incomingOffer);
    }
  }

  public toggleMute(): boolean {
    if (this.localStream) {
      const audioTrack = this.localStream.getAudioTracks()[0];
      if (audioTrack) {
        audioTrack.enabled = !audioTrack.enabled;
        return !audioTrack.enabled;
      }
    }
    return false;
  }

  public async handleOffer(offer: RTCSessionDescriptionInit): Promise<void> {
    try {
      const pc = await this.createPeerConnection();
      await pc.setRemoteDescription(new RTCSessionDescription(offer));
      const answer = await pc.createAnswer();
      await pc.setLocalDescription(answer);
      this.ws.send('call_answer', '', answer, this.sessionID);
      this.onCallStateChange?.('connected');
      this.setupAudioMixer();
      this.startRecording();

      while (this.queuedCandidates.length > 0) {
        const cand = this.queuedCandidates.shift();
        if (cand) await pc.addIceCandidate(cand).catch(() => {});
      }
    } catch (err) {
      console.error('Error handling WebRTC offer:', err);
    }
  }

  public async handleAnswer(answer: RTCSessionDescriptionInit): Promise<void> {
    if (!this.pc) return;
    if (this.pc.signalingState !== 'have-local-offer') {
      console.warn('Ignoring answer because signalingState is:', this.pc.signalingState);
      return;
    }
    try {
      await this.pc.setRemoteDescription(new RTCSessionDescription(answer));
      this.onCallStateChange?.('connected');
      this.setupAudioMixer();
      this.startRecording();

      while (this.queuedCandidates.length > 0) {
        const cand = this.queuedCandidates.shift();
        if (cand) await this.pc.addIceCandidate(cand).catch(() => {});
      }
    } catch (err) {
      console.error('Error setting remote answer description:', err);
    }
  }

  private async handleCandidate(candidate: RTCIceCandidateInit): Promise<void> {
    if (!this.pc || !this.pc.remoteDescription) {
      this.queuedCandidates.push(new RTCIceCandidate(candidate));
      return;
    }
    try {
      await this.pc.addIceCandidate(new RTCIceCandidate(candidate));
    } catch (err) {
      console.error('Error adding ICE candidate:', err);
    }
  }

  /**
   * Web Audio Mixer: Mixes both local microphone and remote incoming audio
   * into a single MediaStream destination for comprehensive 2-way call recording.
   */
  private setupAudioMixer(): void {
    if (typeof window === 'undefined') return;
    try {
      const AudioCtx = window.AudioContext || (window as any).webkitAudioContext;
      if (!AudioCtx) return;

      if (!this.audioContext || this.audioContext.state === 'closed') {
        this.audioContext = new AudioCtx();
      }
      if (this.audioContext.state === 'suspended') {
        this.audioContext.resume().catch(() => {});
      }

      if (!this.mixedDestination) {
        this.mixedDestination = this.audioContext.createMediaStreamDestination();
      }

      // Connect local mic stream
      if (this.localStream && this.localStream.getAudioTracks().length > 0) {
        try {
          const localSource = this.audioContext.createMediaStreamSource(this.localStream);
          localSource.connect(this.mixedDestination);
        } catch (e) {}
      }

      // Connect remote incoming stream
      if (this.remoteStream && this.remoteStream.getAudioTracks().length > 0) {
        try {
          const remoteSource = this.audioContext.createMediaStreamSource(this.remoteStream);
          remoteSource.connect(this.mixedDestination);
        } catch (e) {}
      }
    } catch (err) {
      console.warn('AudioContext mixing initialization failed:', err);
    }
  }

  private speechRecognizer: any = null;
  private callTranscript: string[] = [];
  private currentInterimText: string = '';

  private startSpeechRecognition(): void {
    if (typeof window === 'undefined') return;
    const SpeechRec = (window as any).SpeechRecognition || (window as any).webkitSpeechRecognition;
    if (!SpeechRec) return;

    try {
      if (this.speechRecognizer) {
        try { this.speechRecognizer.stop(); } catch (e) {}
      }

      const recognizer = new SpeechRec();
      recognizer.continuous = true;
      recognizer.interimResults = true;
      recognizer.lang = 'vi-VN';

      recognizer.onresult = (event: any) => {
        let interim = '';
        for (let i = event.resultIndex; i < event.results.length; i++) {
          const text = event.results[i][0]?.transcript?.trim();
          if (text) {
            if (event.results[i].isFinal) {
              if (!this.callTranscript.includes(text)) {
                this.callTranscript.push(text);
                console.log('🎙️ Real-time speech transcript (final):', text);
              }
              this.currentInterimText = '';
            } else {
              interim += text + ' ';
            }
          }
        }
        if (interim.trim()) {
          this.currentInterimText = interim.trim();
          console.log('🎙️ Real-time speech interim:', this.currentInterimText);
        }
      };

      recognizer.onerror = (e: any) => {
        console.warn('SpeechRecognition notice:', e.error);
      };

      recognizer.onend = () => {
        if (!this.isEnding && this.pc && this.pc.connectionState !== 'closed') {
          try { recognizer.start(); } catch (e) {}
        }
      };

      recognizer.start();
      this.speechRecognizer = recognizer;
      console.log('🎙️ Web Speech Recognition started for Vietnamese audio transcription');
    } catch (e) {
      console.warn('SpeechRecognition start failed:', e);
    }
  }

  private startRecording(): void {
    if (this.mediaRecorder && this.mediaRecorder.state === 'recording') {
      return;
    }
    const streamToRecord = this.mixedDestination?.stream || this.localStream;
    if (!streamToRecord || streamToRecord.getAudioTracks().length === 0) return;

    this.recordedChunks = [];
    this.callTranscript = [];
    this.startSpeechRecognition();

    try {
      let mimeType = 'audio/webm;codecs=opus';
      if (typeof MediaRecorder !== 'undefined') {
        if (!MediaRecorder.isTypeSupported(mimeType)) {
          if (MediaRecorder.isTypeSupported('audio/webm')) {
            mimeType = 'audio/webm';
          } else if (MediaRecorder.isTypeSupported('audio/ogg;codecs=opus')) {
            mimeType = 'audio/ogg;codecs=opus';
          } else {
            mimeType = '';
          }
        }
      }

      const options = mimeType ? { mimeType } : undefined;
      this.mediaRecorder = new MediaRecorder(streamToRecord, options);

      this.mediaRecorder.ondataavailable = (event) => {
        if (event.data && event.data.size > 0) {
          this.recordedChunks.push(event.data);
        }
      };

      this.mediaRecorder.onstop = () => {
        if (this.recordingStopResolver) {
          this.recordingStopResolver();
          this.recordingStopResolver = null;
        }
      };

      this.mediaRecorder.start(1000);
      console.log('🎙️ MediaRecorder started recording voice call');
    } catch (err) {
      console.warn('MediaRecorder audio recording not supported on this browser:', err);
    }
  }

  private isEnding = false;

  public async endCall(broadcast: boolean = true, durationSeconds: number = 0): Promise<string | null> {
    if (this.isEnding) return null;
    this.isEnding = true;

    if (this.speechRecognizer) {
      try { this.speechRecognizer.stop(); } catch (e) {}
      this.speechRecognizer = null;
    }

    if (broadcast) {
      this.ws.send('call_end', '', {}, this.sessionID);
    }

    // Stop MediaRecorder and wait for flush
    if (this.mediaRecorder && this.mediaRecorder.state !== 'inactive') {
      try {
        const stopPromise = new Promise<void>((resolve) => {
          this.recordingStopResolver = resolve;
          setTimeout(resolve, 800);
        });
        try { this.mediaRecorder.requestData(); } catch (e) {}
        this.mediaRecorder.stop();
        await stopPromise;
      } catch (e) {}
    }

    if (this.localStream) {
      this.localStream.getTracks().forEach((track) => track.stop());
      this.localStream = null;
    }
    if (this.remoteStream) {
      this.remoteStream.getTracks().forEach((track) => track.stop());
      this.remoteStream = null;
    }

    if (this.audioContext && this.audioContext.state !== 'closed') {
      try { this.audioContext.close(); } catch (e) {}
      this.audioContext = null;
      this.mixedDestination = null;
    }

    if (this.pc) {
      try { this.pc.close(); } catch (e) {}
      this.pc = null;
    }

    this.onCallStateChange?.('ended');

    // Upload recording if available
    let recordingURL: string | null = null;
    if (this.recordedChunks.length > 0) {
      const mime = this.recordedChunks[0]?.type || 'audio/webm';
      const blob = new Blob(this.recordedChunks, { type: mime });
      console.log(`🎙️ Uploading call recording (${blob.size} bytes)...`);
      recordingURL = await this.uploadRecordingBlob(blob, durationSeconds);
    }

    return recordingURL;
  }

  private async uploadRecordingBlob(blob: Blob, durationSeconds: number = 0): Promise<string | null> {
    try {
      const formData = new FormData();
      formData.append('audio', blob, `call_${this.sessionID}_${Date.now()}.webm`);
      formData.append('session_id', this.sessionID);
      if (this.callID) {
        formData.append('call_id', this.callID.toString());
      }
      formData.append('duration_seconds', durationSeconds.toString());
      let fullTranscript = this.callTranscript.join('. ').trim();
      if (this.currentInterimText && !fullTranscript.includes(this.currentInterimText)) {
        fullTranscript = fullTranscript ? `${fullTranscript}. ${this.currentInterimText}` : this.currentInterimText;
      }
      if (fullTranscript) {
        formData.append('transcript', fullTranscript);
        console.log('🎙️ Uploading call transcript to server:', fullTranscript);
      }

      const targetURL = typeof window !== 'undefined' && window.location.port === '3000'
        ? `${window.location.protocol}//${window.location.hostname}:8080/api/voice/upload-recording`
        : '/api/voice/upload-recording';

      let res = await fetch(targetURL, {
        method: 'POST',
        body: formData,
      }).catch(async () => {
        return fetch('/api/voice/upload-recording', {
          method: 'POST',
          body: formData,
        });
      });

      if (res && res.ok) {
        const data = await res.json();
        console.log('✅ Call recording uploaded successfully:', data.recording_url);
        return data.recording_url;
      } else {
        console.error('❌ Upload recording error status:', res?.status);
      }
    } catch (err) {
      console.error('❌ Failed to upload call recording:', err);
    }
    return null;
  }
}

export { WebRTCVoiceManager as WebRTCManager };
