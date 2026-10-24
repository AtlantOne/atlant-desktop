// Presentation-only navigation. Never saves config or reconnects the VPN.
(function () {
  const menu = document.getElementById('settings-menu');
  const back = document.getElementById('settings-back');
  const save = document.getElementById('btn-save-settings');
  const heading = document.getElementById('settings-heading');
  const links = document.querySelectorAll('[data-settings-open]');
  const sections = document.querySelectorAll('[data-settings-section]');
  function show(page, title) {
    if (menu) menu.hidden = page !== null;
    if (back) back.hidden = page === null;
    if (save) save.hidden = page === null || page === 'appearance' || page === 'language' || page === 'advanced';
    const hint = document.querySelector('.settings-apply-hint');
    if (hint) hint.hidden = page === 'language' || page === 'appearance';
    if (heading) heading.textContent = title || tr("Настройки");
    sections.forEach(section => { section.hidden = section.dataset.settingsSection !== page; });
    const scroll = document.querySelector('.settings-scroll-area');
    if (scroll) scroll.scrollTop = 0;
    if (back && page !== null) back.focus();
  }
  links.forEach(link => link.addEventListener('click', () => show(link.dataset.settingsOpen, link.querySelector('strong').textContent)));
  if (back) back.addEventListener('click', () => { show(null); links[0]?.focus(); });
  show(null);
  const routing = document.getElementById('tab-routing');
  const views = document.querySelectorAll('[data-routing-view]');
  function view(name) {
    if (routing) routing.dataset.view = name;
    views.forEach(button => button.setAttribute('aria-pressed', String(button.dataset.routingView === name)));
  }
  views.forEach(button => button.addEventListener('click', () => view(button.dataset.routingView)));
  view('domains');
})();
