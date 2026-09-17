// Native selects remain the value/event source for the form logic.
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
    this.searchable = select.id === 'product';
    this.active = select.selectedIndex;
    this.search = '';
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
    this.menu.hidden = true;
    this.searchInput = document.createElement('input');
    this.searchInput.type = 'search';
    this.searchInput.className = 'select-search';
    this.searchInput.placeholder = select.id === 'product' ? 'Search name or product ID…' : 'Search options…';
    this.searchInput.setAttribute('aria-label', this.searchInput.placeholder.replace('…', ''));
    this.searchInput.autocomplete = 'off';
    this.searchInput.hidden = true;
    this.list = document.createElement('div');
    this.list.className = 'select-list';
    this.list.id = `select-list-${++nextId}`;
    this.list.setAttribute('role', 'listbox');
    this.list.setAttribute('aria-label', this.button.getAttribute('aria-label'));
    this.searchInput.setAttribute('role', 'combobox');
    this.searchInput.setAttribute('aria-autocomplete', 'list');
    this.searchInput.setAttribute('aria-controls', this.list.id);
    this.searchInput.setAttribute('aria-expanded', 'false');
    this.result = document.createElement('div');
    this.result.className = 'select-results';
    this.result.setAttribute('role', 'status');
    this.button.setAttribute('aria-controls', this.list.id);
    this.menu.append(this.result, this.list);
    this.result.hidden = !this.searchable;
    select.before(this.wrapper);
    this.wrapper.append(select, this.button);
    if (this.searchable) this.wrapper.append(this.searchInput);
    this.wrapper.append(this.menu);
    select.hidden = true;
    this.button.onclick = () => this.menu.hidden ? this.open(true) : this.close();
    this.button.onkeydown = event => this.keydown(event);
    this.searchInput.oninput = () => this.filter(this.searchInput.value);
    this.searchInput.onkeydown = event => {
      if (['ArrowDown', 'ArrowUp', 'Enter', 'Escape', 'Tab'].includes(event.key)) this.keydown(event);
    };
    // Contain trackpad/wheel momentum even at the first or last option.
    this.menu.addEventListener('wheel', event => {
      event.preventDefault();
      const unit = event.deltaMode === 1 ? 18 : event.deltaMode === 2 ? this.list.clientHeight : 1;
      this.list.scrollTop += event.deltaY * unit;
    }, { passive: false });
    select.addEventListener('change', () => this.sync());
    this.observer = new MutationObserver(() => this.rebuild());
    this.observer.observe(select, { childList: true, subtree: true, attributes: true });
    this.rebuild();
  }

  rebuild() {
    this.visible = matchingIndices([...this.select.options], this.search);
    this.list.replaceChildren();
    for (const index of this.visible) {
      const option = this.select.options[index];
      const item = document.createElement('button');
      item.type = 'button';
      item.className = 'select-option';
      item.id = `${this.list.id}-option-${index}`;
      item.dataset.index = index;
      item.tabIndex = -1;
      item.setAttribute('role', 'option');
      item.textContent = option.textContent;
      item.title = option.textContent;
      item.onmousedown = event => event.preventDefault();
      item.onclick = () => {
        this.select.selectedIndex = index;
        this.select.dispatchEvent(new Event('change', { bubbles: true }));
        this.close();
        this.button.focus({ preventScroll: true });
      };
      this.list.append(item);
    }
    this.result.textContent = this.search ? `${this.visible.length} ${this.visible.length === 1 ? 'match' : 'matches'}` : 'Type to filter, ↑ ↓ to browse, Enter to select';
    this.sync();
  }

  filter(query) {
    this.search = query;
    this.searchInput.value = query;
    this.rebuild();
    this.list.scrollTop = 0;
    this.highlight(this.visible[0] ?? -1);
  }

  sync() {
    const text = this.select.options[this.select.selectedIndex]?.textContent || 'Select…';
    if (this.button.textContent !== text) this.button.textContent = text;
    this.button.title = this.searchable ? `${text} · Search by name or product ID` : text;
    this.button.disabled = this.select.disabled || !this.select.options.length;
    for (const item of this.list.children) {
      item.setAttribute('aria-selected', String(Number(item.dataset.index) === this.select.selectedIndex));
    }
  }

  open() {
    openControl?.close();
    openControl = this;
    this.menu.hidden = false;
    this.searchInput.hidden = !this.searchable;
    this.wrapper.classList.toggle('searching', this.searchable);
    this.button.setAttribute('aria-expanded', 'true');
    this.searchInput.setAttribute('aria-expanded', 'true');
    const rect = this.button.getBoundingClientRect();
    const below = window.innerHeight - rect.bottom - 12;
    const above = rect.top - 12;
    const flip = below < Math.min(this.menu.scrollHeight, 330) && above > below;
    this.menu.classList.toggle('above', flip);
    this.list.style.maxHeight = `${Math.max(60, Math.min(240, (flip ? above : below) - this.result.offsetHeight - 16))}px`;
    this.highlight(this.visible.includes(this.select.selectedIndex) ? this.select.selectedIndex : this.visible[0] ?? -1);
    (this.searchable ? this.searchInput : this.button).focus({ preventScroll: true });
  }

  close() {
    this.menu.hidden = true;
    this.searchInput.hidden = true;
    this.wrapper.classList.remove('searching');
    this.button.setAttribute('aria-expanded', 'false');
    this.button.removeAttribute('aria-activedescendant');
    this.searchInput.setAttribute('aria-expanded', 'false');
    this.searchInput.removeAttribute('aria-activedescendant');
    this.search = '';
    this.searchInput.value = '';
    this.rebuild();
    if (openControl === this) openControl = undefined;
  }

  highlight(index) {
    this.active = index;
    for (const item of this.list.children) item.classList.toggle('active', Number(item.dataset.index) === index);
    const item = [...this.list.children].find(item => Number(item.dataset.index) === index);
    if (item) {
      this.button.setAttribute('aria-activedescendant', item.id);
      this.searchInput.setAttribute('aria-activedescendant', item.id);
      // Scroll only the option list, never its enclosing page.
      if (item.offsetTop < this.list.scrollTop) this.list.scrollTop = item.offsetTop;
      else if (item.offsetTop + item.offsetHeight > this.list.scrollTop + this.list.clientHeight) this.list.scrollTop = item.offsetTop + item.offsetHeight - this.list.clientHeight;
    } else {
      this.button.removeAttribute('aria-activedescendant');
      this.searchInput.removeAttribute('aria-activedescendant');
    }
  }

  keydown(event) {
    if (event.key === 'Escape' || event.key === 'Tab') {
      if (event.key === 'Tab' && this.searchable) this.button.focus({ preventScroll: true });
      this.close();
      if (event.key === 'Escape') {
        event.preventDefault();
        this.button.focus({ preventScroll: true });
      }
      return;
    }
    if (['ArrowDown', 'ArrowUp', 'Home', 'End'].includes(event.key)) {
      event.preventDefault();
      if (this.menu.hidden) this.open();
      const position = this.visible.indexOf(this.active);
      const next = event.key === 'Home' ? 0 : event.key === 'End' ? this.visible.length - 1 : position + (event.key === 'ArrowUp' ? -1 : 1);
      if (next >= 0 && next < this.visible.length) this.highlight(this.visible[next]);
    } else if (event.key === 'Enter' || (event.key === ' ' && !this.search)) {
      event.preventDefault();
      if (this.menu.hidden) this.open(true);
      else [...this.list.children].find(item => Number(item.dataset.index) === this.active)?.click();
    } else if (this.searchable && (event.key.length === 1 || event.key === 'Backspace') && !event.ctrlKey && !event.metaKey && !event.altKey) {
      event.preventDefault();
      if (this.menu.hidden) this.open();
      this.filter(event.key === 'Backspace' ? this.search.slice(0, -1) : this.search + event.key);
      this.searchInput.focus({ preventScroll: true });
    }
  }
}

if (typeof document !== 'undefined') document.addEventListener('pointerdown', event => {
  if (openControl && !openControl.wrapper.contains(event.target)) openControl.close();
});
if (typeof document !== 'undefined') document.addEventListener('focusin', event => {
  if (openControl && !openControl.wrapper.contains(event.target)) openControl.close();
});

export function matchingIndices(options, query) {
  query = query.trim().toLowerCase();
  const indices = options.flatMap((option, index) => !option.disabled ? [index] : []);
  if (!query) return indices;
  if (/^\d+$/.test(query)) {
    return indices.filter(index => options[index].value.startsWith(query)).sort((a, b) => Number(options[b].value === query) - Number(options[a].value === query));
  }
  const terms = query.split(/\s+/);
  return indices.filter(index => terms.every(term => options[index].textContent.toLowerCase().includes(term)));
}

export function findMatch(options, query) {
  return query.trim() ? matchingIndices(options, query)[0] ?? -1 : -1;
}
