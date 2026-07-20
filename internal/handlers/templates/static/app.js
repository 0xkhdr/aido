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
    const confirmed = confirm(`Delete ${selectedTasks.size} selected tasks?`);
    if (confirmed) {
      bulkAction('delete');
    }
  });

  document.querySelector('[name="priority"]')?.addEventListener('change', (e) => {
    if (e.target.value) {
      bulkAction('change-priority', { priority: e.target.value });
      e.target.value = '';
    }
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

// Initialize on page load
document.addEventListener('DOMContentLoaded', initBulkToolbar);
