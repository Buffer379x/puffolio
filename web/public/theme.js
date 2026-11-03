(function () {
  try {
    var theme = localStorage.getItem('puffolio-theme') || 'system';
    if (!['light', 'dark', 'system'].includes(theme)) theme = 'system';
    document.documentElement.dataset.theme =
      theme === 'system'
        ? matchMedia('(prefers-color-scheme: dark)').matches
          ? 'dark'
          : 'light'
        : theme;
  } catch (_) {
    document.documentElement.dataset.theme = 'light';
  }
})();
