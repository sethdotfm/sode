// Full width: the grid drops the page's width cap and padding. Remembered per browser (`sode-panel-full`).
const KEY = 'sode-panel-full';

export function initFull(button, main) {
  const set = (on) => {
    main.classList.toggle('sode-panels--full', on);
    button.setAttribute('aria-pressed', String(on));
    button.textContent = on ? 'Normal width' : 'Full width';
  };
  let on = false;
  try {
    on = localStorage.getItem(KEY) === '1';
  } catch {
    // Blocked storage only costs the memory of the choice.
  }
  set(on);
  button.addEventListener('click', () => {
    on = !on;
    set(on);
    try {
      localStorage.setItem(KEY, on ? '1' : '0');
    } catch {
      // See above.
    }
  });
}
