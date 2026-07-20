// Search functionality with debounce
function debounce(fn, delay) {
  let timeout;
  return function (...args) {
    clearTimeout(timeout);
    timeout = setTimeout(() => fn.apply(this, args), delay);
  };
}

function getProjectId() {
  return new URLSearchParams(window.location.search).get('project') ||
         window.location.pathname.split('/').pop() || 1;
}

function handleSearch(query) {
  const projectId = getProjectId();
  const trimmedQuery = query.trim();

  if (!trimmedQuery) {
    // Clear search results
    location.reload();
    return;
  }

  fetch(`/api/projects/${projectId}/search?q=${encodeURIComponent(trimmedQuery)}`)
    .then(r => r.json())
    .then(data => {
      const taskList = document.getElementById('task-list');
      const resultInfo = document.getElementById('search-results-info');

      if (data.tasks && data.tasks.length > 0) {
        // Render search results
        const taskHtml = data.tasks.map(task => `
          <li class="${task.done ? 'done' : ''}">
            <input type="checkbox" class="task-select" data-task-id="${task.id}"
              aria-label="Select task for bulk operations">
            <input type="checkbox" ${task.done ? 'checked' : ''}
              hx-post="/tasks/${task.id}/toggle?project=${projectId}" hx-target="#task-list" hx-swap="outerHTML"
              aria-label="Toggle done">
            <span>${task.title}</span>
            <button class="del" hx-delete="/tasks/${task.id}?project=${projectId}" hx-target="#task-list" hx-swap="outerHTML"
              hx-confirm="Delete this task?" aria-label="Delete task">✕</button>
          </li>
        `).join('');

        taskList.innerHTML = taskHtml;
        resultInfo.innerHTML = `Showing ${data.tasks.length} result${data.tasks.length !== 1 ? 's' : ''} for "${trimmedQuery}"`;
        resultInfo.style.display = 'block';
      } else {
        taskList.innerHTML = '<li class="empty-row"><span class="empty"><strong>No results</strong>Try a different search term.</span></li>';
        resultInfo.innerHTML = `No results for "${trimmedQuery}"`;
        resultInfo.style.display = 'block';
      }

      // Re-initialize bulk toolbar after rendering
      initBulkToolbar();
    })
    .catch(err => {
      console.error('Search failed:', err);
      alert('Search failed: ' + err.message);
    });
}

// Bulk operations task selection
const selectedTasks = new Set();

function initBulkToolbar() {
  const checkboxes = document.querySelectorAll('.task-select');
  const toolbar = document.querySelector('.bulk-toolbar');

  checkboxes.forEach(checkbox => {
    checkbox.addEventListener('change', () => {
      const taskId = parseInt(checkbox.dataset.taskId);

      if (checkbox.checked) {
        selectedTasks.add(taskId);
      } else {
        selectedTasks.delete(taskId);
      }

      updateToolbar();
    });
  });

  // Bulk action handlers
  document.querySelector('[data-action="mark-done"]')?.addEventListener('click', () => {
    bulkAction('mark-done');
  });

  document.querySelector('[data-action="delete"]')?.addEventListener('click', () => {
    const dialog = document.getElementById('delete-confirm');
    const msg = document.getElementById('delete-confirm-msg');
    msg.textContent = `Confirm delete ${selectedTasks.size} selected tasks? This cannot be undone.`;
    dialog.showModal();
  });

  document.querySelector('[name="priority"]')?.addEventListener('change', (e) => {
    if (e.target.value) {
      bulkAction('change-priority', { priority: e.target.value });
      e.target.value = '';
    }
  });

  // Delete confirmation dialog buttons
  document.getElementById('delete-cancel')?.addEventListener('click', () => {
    document.getElementById('delete-confirm').close();
  });

  document.getElementById('delete-submit')?.addEventListener('click', () => {
    document.getElementById('delete-confirm').close();
    bulkAction('delete');
  });
}

function updateToolbar() {
  const toolbar = document.querySelector('.bulk-toolbar');
  const count = document.getElementById('bulk-count');

  if (selectedTasks.size > 0) {
    toolbar.style.display = 'block';
    count.textContent = `${selectedTasks.size} tasks selected`;
  } else {
    toolbar.style.display = 'none';
  }
}

function bulkAction(action, options = {}) {
  const projectId = new URLSearchParams(window.location.search).get('project') || 1;
  const payload = {
    task_ids: Array.from(selectedTasks),
    action: action,
    ...options
  };

  fetch(`/api/projects/${projectId}/bulk-actions`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload)
  })
  .then(r => r.json())
  .then(data => {
    if (data.success) {
      clearSelection();
      location.reload();
    } else {
      alert(`Error: ${data.error}`);
    }
  })
  .catch(err => alert(`Bulk action failed: ${err}`));
}

function clearSelection() {
  selectedTasks.clear();
  document.querySelectorAll('.task-select').forEach(cb => cb.checked = false);
  updateToolbar();
}

function quickCreateSuccess(form) {
  form.reset();
  form.querySelector('input[name="title"]').focus();
}

// Tag management
function addTag(taskId, tagName) {
  if (!tagName.trim()) return;

  fetch(`/tasks/${taskId}/tags/add`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name: tagName })
  })
  .then(r => r.json())
  .then(data => {
    if (data.success) {
      const tagList = document.getElementById('tag-list');
      const btn = document.createElement('button');
      btn.type = 'button';
      btn.className = 'tag-badge';
      btn.textContent = `${tagName} ✕`;
      btn.dataset.tagId = data.tag_id;
      btn.dataset.taskId = taskId;
      btn.onclick = () => removeTag(taskId, data.tag_id, btn);
      tagList.appendChild(btn);
      document.getElementById('tag-input').value = '';
    } else {
      alert(`Error: ${data.error}`);
    }
  })
  .catch(err => alert(`Failed to add tag: ${err}`));
}

function removeTag(taskId, tagId, btn) {
  fetch(`/tasks/${taskId}/tags/${tagId}`, { method: 'DELETE' })
  .then(r => r.json())
  .then(data => {
    if (data.success) {
      btn.remove();
    } else {
      alert(`Error: ${data.error}`);
    }
  })
  .catch(err => alert(`Failed to remove tag: ${err}`));
}

function initTags() {
  const tagInput = document.getElementById('tag-input');
  if (!tagInput) return;

  const taskId = tagInput.dataset.taskId;

  tagInput.addEventListener('keydown', (e) => {
    if (e.key === 'Enter') {
      e.preventDefault();
      addTag(taskId, tagInput.value);
    }
  });

  document.querySelectorAll('#tag-list .tag-badge').forEach(btn => {
    btn.onclick = () => removeTag(taskId, btn.dataset.tagId, btn);
  });
}

// Initialize on page load
document.addEventListener('DOMContentLoaded', () => {
  initBulkToolbar();
  initTags();

  // Search box event listener with debounce
  const searchBox = document.getElementById('search-box');
  if (searchBox) {
    searchBox.addEventListener('input', debounce((e) => {
      handleSearch(e.target.value);
    }, 300));
  }
});
