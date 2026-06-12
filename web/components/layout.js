import { renderSidebar, bindSidebar } from './sidebar.js';
export function layout(currentPath, content) {
  return `<div class="layout">${renderSidebar(currentPath)}<main class="content">${content}</main></div>`;
}
export function bindLayout() { bindSidebar(); }
