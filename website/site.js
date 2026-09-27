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
    ? 'Windows 10 (1903+) / 11 · 64-bit · Addon included'
    : 'macOS 14+ · Apple Silicon only · Addon included';
})();
