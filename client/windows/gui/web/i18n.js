// Only explicit UI strings are translated; imported profiles and logs are not.
(function (root) {
  const supported = ['system', 'en', 'ru', 'zh'];
  function resolveLanguage(value, systemLanguage) {
    const selected = supported.includes(value) ? value : 'system';
    const tag = (selected === 'system' ? systemLanguage : selected).toLowerCase().split(/[-_]/)[0];
    return ['en', 'ru', 'zh'].includes(tag) ? tag : 'en';
  }
  function format(template, values) {
    return template.replace(/\{(\d+)\}/g, (match, index) => index < values.length ? String(values[index]) : match);
  }
  function translate(catalog, language, source, ...values) {
    return format(language === 'ru' ? source : catalog[source]?.[language] ?? source, values);
  }
  if (typeof module !== 'undefined') module.exports = {resolveLanguage, format, translate};
  if (!root.document) return;
  let selected = 'system';
  try { selected = localStorage.getItem('atlant-ui-language') || 'system'; } catch {}
  if (supported.includes(root.atlantUiLanguage)) selected = root.atlantUiLanguage;
  const lang = resolveLanguage(selected, navigator.language || 'en');
  const catalog = root.atlantClientLanguages || {};
  root.tr = (source, ...values) => translate(catalog, lang, source, ...values);
  {
    document.documentElement.lang = lang;
    const walker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT);
    const nodes = [];
    while (walker.nextNode()) nodes.push(walker.currentNode);
    for (const node of nodes) {
      if (['SCRIPT', 'STYLE', 'TEXTAREA'].includes(node.parentElement?.tagName)) continue;
      const text = node.textContent.trim();
      if (catalog[text]) node.textContent = node.textContent.replace(text, root.tr(text));
    }
    for (const element of document.querySelectorAll('[title], [placeholder], [aria-label]')) {
      for (const attribute of ['title', 'placeholder', 'aria-label']) {
        const value = element.getAttribute(attribute);
        if (catalog[value]) element.setAttribute(attribute, root.tr(value));
      }
    }
    const picker = document.getElementById('settings-language');
    if (picker) {
      picker.value = supported.includes(selected) ? selected : 'system';
      picker.addEventListener('change', async () => {
        picker.disabled = true;
        try {
          const response = await fetch('/api/ui/preferences', {method:'POST', headers:{'Content-Type':'application/json'}, body:JSON.stringify({language:picker.value})});
          if (!response.ok) throw new Error('Cannot save UI language');
        } catch {
          picker.value = selected;
          picker.disabled = false;
          alert(root.tr('Не удалось сохранить язык'));
          return;
        }
        try { localStorage.setItem('atlant-ui-language', picker.value); } catch {}
        // UI reload only: the native VPN process stays connected.
        location.reload();
      });
    }
  }
})(typeof window === 'undefined' ? globalThis : window);
