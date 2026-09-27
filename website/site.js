// Enhance native audio only after the custom controls are ready.
(() => {
  const template = document.querySelector('#audio-player-template');
  const samples = [...document.querySelectorAll('.audio-card audio')];
  const formatTime = (seconds) => {
    const value = Math.max(0, Math.floor(seconds || 0));
    return `${Math.floor(value / 60)}:${String(value % 60).padStart(2, '0')}`;
  };
  samples.forEach((sample) => {
    sample.addEventListener('play', () => {
      samples.forEach((other) => { if (other !== sample) other.pause(); });
    });
    if (!template) return;
    const player = template.content.firstElementChild.cloneNode(true);
    const label = document.getElementById(sample.getAttribute('aria-labelledby')).textContent;
    const toggle = player.querySelector('.audio-toggle');
    const seek = player.querySelector('.audio-seek');
    const time = player.querySelector('.audio-time');
    const error = player.querySelector('.audio-error');
    player.querySelector('.audio-controls').setAttribute('aria-label', `${label} sample controls`);
    player.querySelector('.audio-download').href = sample.getAttribute('src');
    seek.setAttribute('aria-label', `Seek in ${label} sample`);
    const showError = () => {
      error.hidden = false;
      error.querySelector('[role="status"]').textContent = 'Unable to play. Try again or download the sample.';
    };
    const update = () => {
      const loaded = Number.isFinite(sample.duration) && sample.duration > 0;
      const duration = loaded ? sample.duration : Number(sample.dataset.duration) || 0;
      const current = sample.currentTime || 0;
      const playing = !sample.paused && !sample.ended;
      player.classList.toggle('is-playing', playing);
      toggle.setAttribute('aria-label', `${playing ? 'Pause' : 'Play'} ${label} sample`);
      toggle.title = toggle.getAttribute('aria-label');
      seek.disabled = !loaded;
      seek.max = duration || 1;
      seek.value = current;
      seek.style.setProperty('--progress', `${duration ? Math.min(100, current / duration * 100) : 0}%`);
      seek.setAttribute('aria-valuetext', `${formatTime(current)} of ${formatTime(duration)}`);
      time.textContent = `${formatTime(current)} / ${formatTime(duration)}`;
    };
    toggle.addEventListener('click', async () => {
      if (!sample.paused) {
        sample.pause();
        return;
      }
      error.hidden = true;
      try {
        await sample.play();
      } catch (failure) {
        // A quick pause or another sample starting can interrupt a pending play.
        if (failure.name !== 'AbortError') showError();
      }
    });
    seek.addEventListener('input', () => {
      if (Number.isFinite(sample.duration)) sample.currentTime = Number(seek.value);
      update();
    });
    ['loadedmetadata', 'durationchange', 'timeupdate', 'play', 'playing', 'pause', 'ended', 'seeked'].forEach((event) => {
      sample.addEventListener(event, update);
    });
    sample.addEventListener('error', showError);
    update();
    sample.after(player);
    sample.hidden = true;
    sample.controls = false;
  });
})();

/* OS detection is a convenience, not an architecture check. Keep both downloads
   visible and show the Apple Silicon requirement even on detected Macs. */
(() => {
  const ua = navigator.userAgent || '';
  const platform = navigator.userAgentData?.platform || navigator.platform || '';
  const mobile = /Android|iPhone|iPad|iPod/i.test(ua) || (/Mac/i.test(platform) && navigator.maxTouchPoints > 1);
  let os = null;
  if (!mobile && /Win/i.test(platform)) os = 'windows';
  else if (!mobile && /Mac/i.test(platform)) os = 'mac';
  if (!os) {
    if (!mobile && /Linux/i.test(platform)) document.querySelector('#platform-note').textContent = 'Using Linux? Current downloads support Windows and Apple Silicon Macs.';
    return;
  }
  const card = document.querySelector(`#${os}-download`);
  card.classList.add('recommended');
  card.querySelector('.recommendation').hidden = false;
  document.querySelector('#platform-note').textContent = os === 'windows'
    ? 'Windows detected. Start with the Windows installer below.'
    : 'macOS detected. The Mac download requires an Apple Silicon (M-series) Mac.';
  const link = document.querySelector('#hero-download');
  link.href = card.querySelector('[data-os]').href;
  link.querySelector('.download-label').textContent = os === 'windows' ? 'Download for Windows' : 'Download for macOS';
  link.addEventListener('click', (event) => {
    // Keep the browser's normal download action and modified-click behavior.
    if (event.defaultPrevented || event.button !== 0 || event.ctrlKey || event.metaKey || event.shiftKey || event.altKey) return;
    document.querySelector('#setup').scrollIntoView({
      behavior: window.matchMedia('(prefers-reduced-motion: reduce)').matches ? 'auto' : 'smooth',
      block: 'start',
    });
  });
  document.querySelector('#hero-platform').textContent = os === 'windows'
    ? 'Windows 10/11 · 64-bit · Addon included'
    : 'macOS 14+ · Apple Silicon only · Addon included';
})();
