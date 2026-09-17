// Keep native selects as the value/event source for existing form logic, while
// rendering a consistent control and popup instead of platform-native menus.
const controls = new WeakMap();
let openControl;
let nextId = 0;

export function enhanceSelects(root) {
  for (const select of root.querySelectorAll('select')) {
    if (!controls.has(select)) controls.set(select, new SelectControl(select));
    controls.get(select).sync();
  }
}

export class SelectControl {
  constructor(select) {
    this.select = select;
    this.active = select.selectedIndex;
    this.wrapper = document.createElement('div');
    this.wrapper.className = 'select-control';
    this.button = document.createElement('button');
    this.button.type = 'button';
    this.button.className = 'select-trigger';
    this.button.setAttribute('role', 'combobox');
    this.button.setAttribute('aria-haspopup', 'listbox');
    this.button.setAttribute('aria-expanded', 'false');
    const label = select.closest('label');
    this.button.setAttribute('aria-label', label?.firstChild?.textContent.trim() || select.id);
    this.menu = document.createElement('div');
    this.menu.className = 'select-menu';
    this.menu.id = `select-menu-${++nextId}`;
    this.menu.setAttribute('role', 'listbox');
    this.menu.setAttribute('aria-label', this.button.getAttribute('aria-label'));
    this.menu.hidden = true;
    this.button.setAttribute('aria-controls', this.menu.id);
    select.before(this.wrapper);
    this.wrapper.append(select, this.button, this.menu);
    select.hidden = true;
    this.button.title = 'Type a name or product ID to jump to a match';
    this.button.onclick = () => this.menu.hidden ? this.open() : this.close();
    this.button.onkeydown = event => this.keydown(event);
    select.addEventListener('change', () => this.sync());
    this.observer = new MutationObserver(() => this.rebuild());
    this.observer.observe(select, { childList: true, subtree: true, attributes: true });
    this.rebuild();
  }

  rebuild() {
    this.menu.replaceChildren();
    for (const [index, option] of [...this.select.options].entries()) {
      const item = document.createElement('button');
      item.type = 'button';
      item.className = 'select-option';
      item.id = `${this.menu.id}-option-${index}`;
      item.tabIndex = -1;
      item.setAttribute('role', 'option');
      item.textContent = option.textContent;
      item.title = option.textContent;
      item.disabled = option.disabled;
      // Keep focus on the combobox so keyboard navigation and Escape work.
      item.onmousedown = event => event.preventDefault();
      item.onclick = () => {
        this.select.selectedIndex = index;
        this.select.dispatchEvent(new Event('change', { bubbles: true }));
        this.close();
        this.button.focus();
      };
      this.menu.append(item);
    }
    this.sync();
  }

  sync() {
    const text = this.select.selectedOptions[0]?.textContent || 'Select…';
    if (this.button.textContent !== text) this.button.textContent = text;
    this.button.title = `${text} · Type a name or product ID to find a match`;
    this.button.disabled = this.select.disabled || !this.select.options.length;
    for (const [index, item] of [...this.menu.children].entries()) {
      item.setAttribute('aria-selected', String(index === this.select.selectedIndex));
    }
  }

  open() {
    openControl?.close();
    openControl = this;
    this.button.focus({ preventScroll: true });
    this.menu.hidden = false;
    this.button.setAttribute('aria-expanded', 'true');
    // Flip above the control if the popup would otherwise leave the viewport.
    this.menu.classList.toggle('above', this.button.getBoundingClientRect().bottom + Math.min(this.menu.scrollHeight, 280) > window.innerHeight - 12);
    this.highlight(Math.max(0, this.select.selectedIndex));
  }

  close() {
    this.menu.hidden = true;
    this.button.setAttribute('aria-expanded', 'false');
    this.button.removeAttribute('aria-activedescendant');
    this.search = '';
    this.lastSearch = 0;
    if (openControl === this) openControl = undefined;
  }

  highlight(index) {
    this.active = index;
    for (const [i, item] of [...this.menu.children].entries()) item.classList.toggle('active', i === index);
    const item = this.menu.children[index];
    if (item) {
      this.button.setAttribute('aria-activedescendant', item.id);
      item.scrollIntoView({ block: 'nearest' });
    }
  }

  keydown(event) {
    const options = [...this.select.options];
    if (event.key === 'Escape' || event.key === 'Tab') {
      this.close();
      if (event.key === 'Escape') event.preventDefault();
      return;
    }
    if (['ArrowDown', 'ArrowUp', 'Home', 'End'].includes(event.key)) {
      event.preventDefault();
      if (this.menu.hidden) this.open();
      const direction = event.key === 'ArrowUp' || event.key === 'End' ? -1 : 1;
      let index = event.key === 'Home' ? 0 : event.key === 'End' ? options.length - 1 : this.active + direction;
      while (index >= 0 && index < options.length && options[index].disabled) index += direction;
      if (index >= 0 && index < options.length) this.highlight(index);
    } else if (event.key === 'Enter' || (event.key === ' ' && !this.search)) {
      event.preventDefault();
      if (this.menu.hidden) this.open();
      else this.menu.children[this.active]?.click();
    } else if ((event.key.length === 1 || event.key === 'Backspace') && !event.ctrlKey && !event.metaKey && !event.altKey) {
      event.preventDefault();
      if (this.menu.hidden) this.open();
      const now = Date.now();
      const previous = now - (this.lastSearch || 0) < 1200 ? this.search || '' : '';
      this.search = event.key === 'Backspace' ? previous.slice(0, -1) : previous + event.key;
      this.lastSearch = now;
      const index = findMatch(options, this.search);
      if (index >= 0) this.highlight(index);
    }
  }
}

if (typeof document !== 'undefined') document.addEventListener('pointerdown', event => {
  if (openControl && !openControl.wrapper.contains(event.target)) openControl.close();
});
if (typeof document !== 'undefined') document.addEventListener('focusin', event => {
  if (openControl && !openControl.wrapper.contains(event.target)) openControl.close();
});

// Prefer exact IDs before prefix matches; registry names can contain a brand
// prefix, so names match anywhere rather than only at the first character.
export function findMatch(options, query) {
  query = query.trim().toLowerCase();
  if (!query) return -1;
  const enabled = option => !option.disabled;
  if (/^\d+$/.test(query)) {
    const exact = options.findIndex(option => enabled(option) && option.value === query);
    if (exact >= 0) return exact;
    return options.findIndex(option => enabled(option) && option.value.startsWith(query));
  }
  return options.findIndex(option => enabled(option) && option.textContent.toLowerCase().includes(query));
}
