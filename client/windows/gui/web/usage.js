(function (root) {
  const tr = root.tr || (text => text);
  function bytes(n) {
    const units = [tr("Б"), tr("КиБ"), tr("МиБ"), tr("ГиБ"), tr("ТиБ")];
    let i = 0;
    while (n >= 1024 && i < units.length - 1) { n /= 1024; i++; }
    return n.toLocaleString(root.document?.documentElement?.lang || 'ru-RU', {maximumFractionDigits: 2}) + ' ' + units[i];
  }
  function count(n) { return typeof n === 'number' && Number.isFinite(n) && n > 0 ? n : 0; }
  function summary(sub) {
    const used = count(sub.upload) + count(sub.download), total = count(sub.total);
    if (!total) return {finite: false, percent: 0, text: tr("Использовано ") + bytes(used) + tr(" · безлимит")};
    return {finite: true, percent: Math.min(100, used / total * 100),
      text: tr("Осталось ") + bytes(Math.max(0, total - used)) + tr(" из ") + bytes(total) + tr(" · использовано ") + bytes(used)};
  }
  const api = {summary};
  if (typeof module !== 'undefined') module.exports = api;
  else root.AtlantUsage = api;
})(typeof window !== 'undefined' ? window : globalThis);
