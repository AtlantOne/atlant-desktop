// Appearance is independent of VPN configuration and survives blocked storage.
(function () {
  const media = typeof matchMedia === 'function' ? matchMedia('(prefers-color-scheme: dark)') : {matches:false};
  let saved;
  try { saved = localStorage.getItem('atlant-theme'); } catch (_) {}
  let preference = saved === 'light' || saved === 'dark' ? saved : 'system';
  const root = document.documentElement;
  let button, select;
  function apply() {
    const dark = preference === 'dark' || (preference === 'system' && media.matches);
    if (dark) root.setAttribute('data-theme', 'dark');
    else root.removeAttribute('data-theme');
    if (button) {
      button.classList.toggle('active', dark);
      button.setAttribute('aria-checked', String(dark));
    }
    if (select) select.value = preference;
  }
  function choose(value) {
    preference = ['system','light','dark'].includes(value) ? value : 'system';
    try { localStorage.setItem('atlant-theme', preference); } catch (_) {}
    apply();
  }
  apply();
  function bind() {
    button = document.getElementById('btn-theme-toggle');
    select = document.getElementById('settings-theme');
    if (button) {
      button.setAttribute('role', 'switch');
      button.setAttribute('aria-label', typeof tr === 'function' ? tr('Тёмная тема') : 'Тёмная тема');
      button.addEventListener('click', () => choose(root.getAttribute('data-theme') === 'dark' ? 'light' : 'dark'));
    }
    if (select) select.addEventListener('change', () => choose(select.value));
    apply();
  }
  media.addEventListener?.('change', () => { if (preference === 'system') apply(); });
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', bind, {once:true});
  else bind();
})();
