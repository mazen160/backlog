const { test, expect } = require('@playwright/test');

// ─── helpers ─────────────────────────────────────────────────────────────────

async function waitForApp(page) {
  // Page is ready when the task table (or empty state) is visible
  await page.waitForSelector('.page-title', { timeout: 8000 });
}

async function selectProject(page, alias) {
  await page.selectOption('#project-select', alias);
  await page.waitForTimeout(200);
}

// ─── boot ────────────────────────────────────────────────────────────────────

test('page loads and shows tasks view', async ({ page }) => {
  await page.goto('/');
  await waitForApp(page);

  await expect(page.locator('.page-title')).toHaveText('Tasks');
  await expect(page.locator('.sidebar-item.active')).toHaveText(/Tasks/);
});

test('project selector is populated', async ({ page }) => {
  await page.goto('/');
  await waitForApp(page);

  const options = await page.locator('#project-select option').allTextContents();
  expect(options).toContain('Alpha');
  expect(options).toContain('Beta');
  // "All projects" option
  expect(options[0]).toMatch(/All projects/i);
});

// ─── tasks list ──────────────────────────────────────────────────────────────

test('tasks list shows correct rows for project', async ({ page }) => {
  await page.goto('/');
  await waitForApp(page);
  await selectProject(page, 'alpha');

  const rows = page.locator('.task-row');
  await expect(rows).toHaveCount(3);

  const titles = await rows.locator('td:nth-child(2)').allTextContents();
  expect(titles).toContain('Fix login bug');
  expect(titles).toContain('Add dark mode');
  expect(titles).toContain('Write unit tests');
});

test('task list shows ref numbers', async ({ page }) => {
  await page.goto('/');
  await waitForApp(page);
  await selectProject(page, 'alpha');

  const refs = await page.locator('.task-row td:first-child').allTextContents();
  expect(refs.some(r => r.startsWith('TASK-'))).toBe(true);
});

test('status badge is visible', async ({ page }) => {
  await page.goto('/');
  await waitForApp(page);
  await selectProject(page, 'alpha');

  const badges = await page.locator('.badge-doing, .badge-todo, .badge-done').count();
  expect(badges).toBeGreaterThan(0);
});

test('status filter works', async ({ page }) => {
  await page.goto('/');
  await waitForApp(page);
  await selectProject(page, 'alpha');

  await page.selectOption('#filter-status', 'doing');
  await page.waitForTimeout(300);

  const rows = page.locator('.task-row');
  await expect(rows).toHaveCount(1);
  await expect(rows.locator('td:nth-child(2)')).toHaveText('Fix login bug');
});

test('type filter works', async ({ page }) => {
  await page.goto('/');
  await waitForApp(page);
  await selectProject(page, 'alpha');

  await page.selectOption('#filter-type', 'bug');
  await page.waitForTimeout(300);

  const rows = page.locator('.task-row');
  await expect(rows).toHaveCount(1);
  await expect(rows.locator('td:nth-child(2)')).toHaveText('Fix login bug');
});

test('search filter works', async ({ page }) => {
  await page.goto('/');
  await waitForApp(page);
  await selectProject(page, 'alpha');

  await page.fill('#filter-search', 'dark');
  await page.waitForTimeout(500);

  const rows = page.locator('.task-row');
  await expect(rows).toHaveCount(1);
  await expect(rows.locator('td:nth-child(2)')).toHaveText('Add dark mode');
});

test('clear filters button appears and works', async ({ page }) => {
  await page.goto('/');
  await waitForApp(page);
  await selectProject(page, 'alpha');

  await page.selectOption('#filter-status', 'doing');
  await page.waitForTimeout(300);
  await expect(page.locator('#btn-clear-filters')).toBeVisible();

  await page.click('#btn-clear-filters');
  await page.waitForTimeout(300);

  await expect(page.locator('.task-row')).toHaveCount(3);
  await expect(page.locator('#btn-clear-filters')).not.toBeVisible();
});

test('empty state shown when no tasks match filter', async ({ page }) => {
  await page.goto('/');
  await waitForApp(page);
  await selectProject(page, 'alpha');

  await page.selectOption('#filter-status', 'done');
  await page.waitForTimeout(300);

  await expect(page.locator('.empty-text')).toContainText('No tasks match');
});

// ─── new task modal ───────────────────────────────────────────────────────────

test('new task modal opens via button', async ({ page }) => {
  await page.goto('/');
  await waitForApp(page);

  await page.click('#btn-new-task');
  await expect(page.locator('.modal-title')).toHaveText('New Task');
  await expect(page.locator('#new-task-title')).toBeVisible();
});

test('new task modal opens via N shortcut', async ({ page }) => {
  await page.goto('/');
  await waitForApp(page);

  await page.keyboard.press('n');
  await expect(page.locator('.modal-title')).toHaveText('New Task');
});

test('new task modal closes with Escape', async ({ page }) => {
  await page.goto('/');
  await waitForApp(page);

  await page.click('#btn-new-task');
  await expect(page.locator('.modal-overlay')).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(page.locator('.modal-overlay')).not.toBeVisible();
});

test('new task modal closes on backdrop click', async ({ page }) => {
  await page.goto('/');
  await waitForApp(page);

  await page.click('#btn-new-task');
  await expect(page.locator('.modal-overlay')).toBeVisible();
  await page.mouse.click(10, 10); // click outside modal
  await expect(page.locator('.modal-overlay')).not.toBeVisible();
});

test('creating a task requires title', async ({ page }) => {
  await page.goto('/');
  await waitForApp(page);
  await selectProject(page, 'alpha');

  await page.click('#btn-new-task');
  await page.click('#btn-create-task');
  // Toast error should appear
  await expect(page.locator('.toast-error')).toBeVisible();
  // Modal should still be open
  await expect(page.locator('.modal-overlay')).toBeVisible();
});

test('can create a new task', async ({ page }) => {
  await page.goto('/');
  await waitForApp(page);
  await selectProject(page, 'alpha');

  const initialCount = await page.locator('.task-row').count();

  await page.click('#btn-new-task');
  await page.selectOption('#new-task-project', 'alpha');
  await page.fill('#new-task-title', 'E2E test task');
  await page.selectOption('#new-task-type', 'task');
  await page.click('#btn-create-task');

  await page.waitForTimeout(500);
  await expect(page.locator('.toast-success').first()).toBeVisible();

  const newCount = await page.locator('.task-row').count();
  expect(newCount).toBe(initialCount + 1);

  const titles = await page.locator('.task-row td:nth-child(2)').allTextContents();
  expect(titles).toContain('E2E test task');
});

test('create task with Ctrl+Enter shortcut', async ({ page }) => {
  await page.goto('/');
  await waitForApp(page);
  await selectProject(page, 'alpha');

  await page.click('#btn-new-task');
  await page.fill('#new-task-title', 'Shortcut task');
  await page.keyboard.press('Control+Enter');

  await page.waitForTimeout(500);
  await expect(page.locator('.toast-success').first()).toBeVisible();
});

// ─── task detail ─────────────────────────────────────────────────────────────

test('clicking a task row opens detail view', async ({ page }) => {
  await page.goto('/');
  await waitForApp(page);
  await selectProject(page, 'alpha');

  await page.click('.task-row:first-child');
  await expect(page.locator('.detail-grid')).toBeVisible();
  await expect(page.locator('#back-to-tasks')).toBeVisible();
});

test('task detail shows correct metadata', async ({ page }) => {
  await page.goto('/');
  await waitForApp(page);
  await selectProject(page, 'alpha');

  // Click task "Write unit tests" which has a description
  const row = page.locator('.task-row', { hasText: 'Write unit tests' });
  await row.click();

  await expect(page.locator('.page-title')).toContainText('Write unit tests');
  await expect(page.locator('.detail-grid')).toBeVisible();
  // Description card should be visible
  await expect(page.locator('.markdown')).toBeVisible();
  await expect(page.locator('.markdown')).toContainText('Cover auth module');
});

test('back button returns to task list', async ({ page }) => {
  await page.goto('/');
  await waitForApp(page);
  await selectProject(page, 'alpha');

  await page.click('.task-row:first-child');
  await page.click('#back-to-tasks');
  await expect(page.locator('.page-title')).toHaveText('Tasks');
});

test('status dropdown changes task status', async ({ page }) => {
  await page.goto('/');
  await waitForApp(page);
  await selectProject(page, 'alpha');

  const row = page.locator('.task-row', { hasText: 'Add dark mode' });
  await row.click();
  await expect(page.locator('#status-select')).toHaveValue('todo');

  await page.selectOption('#status-select', 'doing');
  await page.waitForTimeout(500);
  await expect(page.locator('.toast-success').first()).toBeVisible();
  await expect(page.locator('#status-select')).toHaveValue('doing');

  // Revert for other tests
  await page.selectOption('#status-select', 'todo');
  await page.waitForTimeout(300);
});

test('edit task modal opens and saves', async ({ page }) => {
  await page.goto('/');
  await waitForApp(page);
  await selectProject(page, 'alpha');

  const row = page.locator('.task-row', { hasText: 'Add dark mode' });
  await row.click();
  await page.click('#btn-edit-task');

  await expect(page.locator('.modal-title')).toHaveText('Edit Task');
  await expect(page.locator('#edit-title')).toHaveValue('Add dark mode');

  await page.fill('#edit-title', 'Add dark mode (updated)');
  await page.click('#btn-save-edit');

  await page.waitForTimeout(500);
  await expect(page.locator('.toast-success').first()).toBeVisible();
  await expect(page.locator('.page-title')).toContainText('Add dark mode (updated)');

  // Revert
  await page.click('#btn-edit-task');
  await page.fill('#edit-title', 'Add dark mode');
  await page.click('#btn-save-edit');
  await page.waitForTimeout(300);
});

test('can add a comment', async ({ page }) => {
  await page.goto('/');
  await waitForApp(page);
  await selectProject(page, 'alpha');

  const row = page.locator('.task-row', { hasText: 'Fix login bug' });
  await row.click();

  await page.fill('#new-comment', 'Reproduced on v1.2');
  await page.click('#btn-add-comment');

  await page.waitForTimeout(500);
  await expect(page.locator('.toast-success').first()).toBeVisible();
  await expect(page.locator('.comment-item')).toContainText('Reproduced on v1.2');
});

test('can add a plan', async ({ page }) => {
  await page.goto('/');
  await waitForApp(page);
  await selectProject(page, 'alpha');

  const row = page.locator('.task-row', { hasText: 'Fix login bug' });
  await row.click();
  await page.click('#btn-add-plan');

  await page.fill('#plan-title', 'Remediation Plan');
  await page.fill('#plan-body', '## Steps\n1. Fix the bug\n2. Add tests');
  await page.click('#btn-save-plan');

  await page.waitForTimeout(500);
  await expect(page.locator('.toast-success').first()).toBeVisible();
  await expect(page.locator('.plan-item')).toContainText('Remediation Plan');
});

// ─── docs ────────────────────────────────────────────────────────────────────

test('docs page requires project selection', async ({ page }) => {
  await page.goto('/');
  await waitForApp(page);

  // Select "All projects" (empty value) which clears state.project
  await page.selectOption('#project-select', '');
  await page.waitForTimeout(200);
  await page.click('[data-page="docs"]');
  await page.waitForTimeout(300);

  await expect(page.locator('.empty-text')).toContainText('Select a project');
});

test('docs page lists documents', async ({ page }) => {
  await page.goto('/');
  await waitForApp(page);
  await selectProject(page, 'alpha');

  await page.click('[data-page="docs"]');
  await page.waitForTimeout(300);

  await expect(page.locator('.page-title')).toHaveText('Docs');
  await expect(page.locator('table td').first()).toBeVisible();
  await expect(page.locator('a.doc-link')).toContainText('Architecture Overview');
});

test('can create a new doc', async ({ page }) => {
  await page.goto('/');
  await waitForApp(page);
  await selectProject(page, 'alpha');

  await page.click('[data-page="docs"]');
  await page.waitForTimeout(200);

  await page.click('#btn-new-doc');
  await page.fill('#doc-title', 'E2E Test Doc');
  await page.fill('#doc-body', '# Heading\n\nSome content here.');
  await page.click('#btn-save-doc');

  await page.waitForTimeout(500);
  await expect(page.locator('.toast-success').first()).toBeVisible();
  await expect(page.locator('a.doc-link').filter({ hasText: /^E2E Test Doc$/ })).toBeVisible();
});

test('can view doc content', async ({ page }) => {
  await page.goto('/');
  await waitForApp(page);
  await selectProject(page, 'alpha');

  await page.click('[data-page="docs"]');
  await page.waitForTimeout(200);

  await page.click('a.doc-link:has-text("Architecture Overview")');
  await expect(page.locator('.modal-overlay')).toBeVisible();
  await expect(page.locator('.modal-title')).toContainText('Architecture Overview');
});

test('can edit a doc', async ({ page }) => {
  await page.goto('/');
  await waitForApp(page);
  await selectProject(page, 'alpha');

  await page.click('[data-page="docs"]');
  await page.waitForTimeout(200);

  await page.click('.doc-edit:first-of-type');
  await expect(page.locator('.modal-title')).toContainText('Edit:');

  const titleVal = await page.locator('#edit-doc-title').inputValue();
  await page.fill('#edit-doc-title', titleVal + ' (edited)');
  await page.click('#btn-update-doc');

  await page.waitForTimeout(500);
  await expect(page.locator('.toast-success').first()).toBeVisible();
});

test('doc delete requires confirmation', async ({ page }) => {
  await page.goto('/');
  await waitForApp(page);
  await selectProject(page, 'alpha');

  await page.click('[data-page="docs"]');
  await page.waitForTimeout(200);

  // Create a doc to delete
  await page.click('#btn-new-doc');
  await page.fill('#doc-title', 'Delete Me');
  await page.fill('#doc-body', 'temp');
  await page.click('#btn-save-doc');
  await page.waitForTimeout(400);

  // Click delete on it
  const row = page.locator('tr', { hasText: 'Delete Me' });
  await row.locator('.doc-del').click();

  // Confirm modal appears
  await expect(page.locator('.modal-overlay')).toBeVisible();
  await expect(page.locator('.modal-body p')).toContainText('Delete this document');

  // Cancel — doc still exists
  await page.click('#btn-cancel');
  await expect(page.locator('a.doc-link', { hasText: 'Delete Me' })).toBeVisible();

  // Delete for real
  await row.locator('.doc-del').click();
  await page.click('#btn-confirm');
  await page.waitForTimeout(500);
  await expect(page.locator('.toast-success').last()).toBeVisible();
  await expect(page.locator('a.doc-link', { hasText: 'Delete Me' })).not.toBeVisible();
});

// ─── memory ──────────────────────────────────────────────────────────────────

test('memory page requires project selection', async ({ page }) => {
  await page.goto('/');
  await waitForApp(page);

  await page.selectOption('#project-select', '');
  await page.waitForTimeout(200);
  await page.click('[data-page="memory"]');
  await page.waitForTimeout(300);

  await expect(page.locator('.empty-text')).toContainText('Select a project');
});

test('memory page shows entries', async ({ page }) => {
  await page.goto('/');
  await waitForApp(page);
  await selectProject(page, 'alpha');

  await page.click('[data-page="memory"]');
  await page.waitForTimeout(300);

  await expect(page.locator('.page-title')).toHaveText('Memory');
  await expect(page.locator('.memory-card').first()).toBeVisible();
  await expect(page.locator('.memory-body').first()).toBeVisible();
  await expect(page.locator('.memory-body', { hasText: 'Decided to use SQLite' })).toBeVisible();
});

test('memory tag chips are clickable and filter entries', async ({ page }) => {
  await page.goto('/');
  await waitForApp(page);
  await selectProject(page, 'alpha');

  await page.click('[data-page="memory"]');
  await page.waitForTimeout(300);

  // Click the "decision" tag chip
  await page.click('.tag-chip:has-text("decision")');
  await page.waitForTimeout(300);

  // Filter input should reflect the tag
  await expect(page.locator('#mem-tag-filter')).toHaveValue('decision');
  await expect(page.locator('.memory-card')).toHaveCount(1);
});

test('memory tag filter input works', async ({ page }) => {
  await page.goto('/');
  await waitForApp(page);
  await selectProject(page, 'alpha');

  await page.click('[data-page="memory"]');
  await page.waitForTimeout(300);

  await page.fill('#mem-tag-filter', 'arch');
  await page.waitForTimeout(500);

  await expect(page.locator('.memory-card')).toHaveCount(1);
});

test('can add a memory entry', async ({ page }) => {
  await page.goto('/');
  await waitForApp(page);
  await selectProject(page, 'alpha');

  await page.click('[data-page="memory"]');
  await page.waitForTimeout(200);

  await page.click('#btn-new-memory');
  await page.fill('#mem-body', 'New memory entry from E2E test');
  await page.fill('#mem-tags', 'test,e2e');
  await page.click('#btn-save-memory');

  await page.waitForTimeout(500);
  await expect(page.locator('.toast-success').first()).toBeVisible();
  await expect(page.locator('.memory-body').filter({ hasText: /^New memory entry from E2E test$/ })).toBeVisible();
});

test('can append to a memory entry', async ({ page }) => {
  await page.goto('/');
  await waitForApp(page);
  await selectProject(page, 'alpha');

  await page.click('[data-page="memory"]');
  await page.waitForTimeout(300);

  await page.click('.mem-append:first-of-type');
  await expect(page.locator('.modal-title')).toHaveText('Append to Memory');
  await expect(page.locator('.memory-preview')).toBeVisible();

  await page.fill('#mem-append-text', 'Appended line from test');
  await page.click('#btn-do-append');

  await page.waitForTimeout(500);
  await expect(page.locator('.toast-success').first()).toBeVisible();

  const bodies = await page.locator('.memory-body').allTextContents();
  expect(bodies.some(b => b.includes('Appended line from test'))).toBe(true);
});

test('memory delete requires confirmation', async ({ page }) => {
  await page.goto('/');
  await waitForApp(page);
  await selectProject(page, 'alpha');

  await page.click('[data-page="memory"]');
  await page.waitForTimeout(300);

  // Add a throwaway entry
  await page.click('#btn-new-memory');
  await page.fill('#mem-body', 'Entry to delete');
  await page.click('#btn-save-memory');
  await page.waitForTimeout(400);

  const card = page.locator('.memory-card', { hasText: 'Entry to delete' });
  await card.locator('.mem-del').click();

  await expect(page.locator('.modal-overlay')).toBeVisible();
  await expect(page.locator('.modal-body p')).toContainText('Delete this memory entry');

  // Cancel first
  await page.click('#btn-cancel');
  await expect(card).toBeVisible();

  // Then confirm delete
  await card.locator('.mem-del').click();
  await page.click('#btn-confirm');
  await page.waitForTimeout(500);
  await expect(page.locator('.toast-success').last()).toBeVisible();
  await expect(page.locator('.memory-card', { hasText: 'Entry to delete' })).not.toBeVisible();
});

// ─── projects page ────────────────────────────────────────────────────────────

test('projects page lists all projects', async ({ page }) => {
  await page.goto('/');
  await waitForApp(page);

  await page.click('[data-page="projects"]');
  await page.waitForTimeout(200);

  await expect(page.locator('.page-title')).toHaveText('Projects');
  await expect(page.locator('.proj-row')).toHaveCount(2);

  const names = await page.locator('.proj-row td:nth-child(2)').allTextContents();
  expect(names.some(n => n.includes('Alpha'))).toBe(true);
  expect(names.some(n => n.includes('Beta'))).toBe(true);
});

test('clicking a project row navigates to its tasks', async ({ page }) => {
  await page.goto('/');
  await waitForApp(page);

  await page.click('[data-page="projects"]');
  await page.waitForTimeout(200);
  await page.click('.proj-row:has-text("Alpha")');
  await page.waitForTimeout(300);

  await expect(page.locator('.page-title')).toHaveText('Tasks');
  await expect(page.locator('#project-select')).toHaveValue('alpha');
});

// ─── sidebar navigation ───────────────────────────────────────────────────────

test('sidebar navigation switches pages', async ({ page }) => {
  await page.goto('/');
  await waitForApp(page);

  await page.click('[data-page="docs"]');
  await expect(page.locator('.sidebar-item.active')).toContainText('Docs');

  await page.click('[data-page="memory"]');
  await expect(page.locator('.sidebar-item.active')).toContainText('Memory');

  await page.click('[data-page="tasks"]');
  await expect(page.locator('.sidebar-item.active')).toContainText('Tasks');
});

test('changing project resets memory tag filter', async ({ page }) => {
  await page.goto('/');
  await waitForApp(page);
  await selectProject(page, 'alpha');

  await page.click('[data-page="memory"]');
  await page.waitForTimeout(200);
  await page.fill('#mem-tag-filter', 'arch');
  await page.waitForTimeout(400);

  // Switch project
  await selectProject(page, 'beta');
  await page.waitForTimeout(200);

  await page.click('[data-page="memory"]');
  await page.waitForTimeout(200);

  // Tag filter should be cleared
  await expect(page.locator('#mem-tag-filter')).toHaveValue('');
});
