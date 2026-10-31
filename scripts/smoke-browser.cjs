#!/usr/bin/env node
// Requires Playwright from PLAYWRIGHT_MODULE (or the normal Node module path).
const assert = require('node:assert/strict');
const path = require('node:path');
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright');

(async () => {
 const baseURL = process.env.SMOKE_BASE_URL || 'http://127.0.0.1:8090';
 assert.ok(['127.0.0.1', 'localhost', '[::1]'].includes(new URL(baseURL).hostname), 'local host only');
 const browser = await chromium.launch({headless: true, executablePath: process.env.CHROMIUM_EXECUTABLE || undefined});
 const created = new Set();
 const checks = [];
 const scopes = new Map();
 let page;
 const getJSON = async endpoint => {
  const response = await page.request.get(baseURL + endpoint);
  assert.equal(response.status(), 200);
  return response.json();
 };
 const scopeItems = async accountId => {
  const items = [];
  for (let number = 1; ; number++) {
   const result = await getJSON('/api/v1/transactions?limit=200&page=' + number + '&account_id=' + encodeURIComponent(accountId));
   assert.ok(result.items.every(tx => tx.account_id === accountId), 'cleanup must stay within registered account scope');
   items.push(...result.items);
   if (items.length >= result.pagination.total) return items;
   assert.ok(result.items.length, 'incomplete cleanup pagination');
  }
 };
 const cleanup = async () => {
  for (const [accountId, baseline] of scopes) {
   for (let attempt = 0; attempt < 3; attempt++) {
    try {
     const ids = (await scopeItems(accountId)).map(tx => tx.id).filter(id => !baseline.has(id));
     if (ids.length) {
      const response = await page.request.post(baseURL + '/api/v1/transactions/batch-delete', {data: {ids}});
      assert.equal(response.status(), 204);
     }
     assert.ok((await scopeItems(accountId)).every(tx => baseline.has(tx.id)), 'cleanup deletion must be confirmed by read');
     break;
    } catch (error) {
     if (attempt === 2) throw new Error('RECOVERY required: base=' + baseURL + ' account_id=' + accountId +
      ' baseline_ids=' + JSON.stringify([...baseline]) + ' known_ids=' + JSON.stringify([...created]) +
      '; query this scope and confirm deletion after network recovery: ' + error.message);
    }
   }
  }
 };
 try {
  page = await browser.newPage({viewport: {width: 1440, height: 1000}});
  const errors = [];
  page.on('pageerror', error => errors.push(error.message));
  const initial = await getJSON('/api/v1/transactions?limit=200');
  const session = await getJSON('/api/v1/session');
  assert.equal(session.environment, 'development');
  assert.equal(session.user_name, 'Synthetic Admin');
  assert.equal(initial.pagination.total, 0, 'browser smoke requires an empty synthetic tenant');
  await page.goto(baseURL);
  await page.getByText('Synthetic Admin', {exact: true}).waitFor();
  await page.getByText('Nenhuma transação encontrada', {exact: true}).waitFor();
  assert.match(await page.locator('body').innerText(), /\(0 registros no total\)/, 'pagination must display the API total');
  checks.push('dashboard, session and empty pagination');
  const accounts = (await getJSON('/api/v1/accounts')).items;
  const account = accounts.find(item => item.type === 'CHECKING');
  const categories = (await getJSON('/api/v1/categories')).items;
  const category = categories.find(item => item.type === 'EXPENSE');
  // Before mutation: this empty synthetic tenant must have no concurrent editors.
  const baseline = new Set((await scopeItems(account.id)).map(tx => tx.id));
  assert.equal(baseline.size, 0, 'empty upload account scope required');
  scopes.set(account.id, baseline);
  for (let attempt = 0; attempt < 2; attempt++) {
   await page.getByRole('button', {name: 'Importar CSV', exact: true}).click();
   await page.locator('form select').selectOption(account.id);
   await page.locator('input[type=file]').setInputFiles(path.resolve(__dirname, '../test/fixtures/extrato_itau.csv'));
   const [response] = await Promise.all([
    page.waitForResponse(item => item.url().endsWith('/transactions/import/csv') && item.request().method() === 'POST'),
    page.getByRole('button', {name: 'Importar Extrato', exact: true}).click(),
   ]);
   const current = await scopeItems(account.id);
   current.forEach(item => created.add(item.id));
   assert.equal(response.status(), 200);
   const imported = await response.json();
   assert.equal(imported.inserted, attempt === 0 ? 100 : 0);
   assert.equal(imported.duplicates, attempt === 0 ? 0 : 100);
   await page.getByRole('button', {name: 'Concluir', exact: true}).click();
   await page.getByText(/\(100 registros no total\)/).waitFor();
  }
  checks.push('CSV import and duplicate reimport through UI');
  await page.getByRole('button', {name: 'Próxima', exact: true}).click();
  await page.getByText(/Mostrando página 2 de 2/).waitFor();
  await page.getByRole('button', {name: 'Anterior', exact: true}).click();
  await page.getByText(/Mostrando página 1 de 2/).waitFor();
  checks.push('pagination navigation');
  const search = page.getByPlaceholder('Buscar por descrição, notas...');
  await search.fill('SUPERMERCADO');
  await page.waitForFunction(() => {
   const rows = [...document.querySelectorAll('tbody tr')];
   return rows.length > 0 && rows.every(row => row.textContent.includes('SUPERMERCADO'));
  });
  await page.getByRole('button', {name: 'Limpar Filtros', exact: true}).click();
  await page.getByText(/\(100 registros no total\)/).waitFor();
  checks.push('description filter and reset');
  const [sortedResponse] = await Promise.all([
   page.waitForResponse(item => new URL(item.url()).pathname === '/api/v1/transactions' && item.url().includes('sort_by=amount')),
   page.locator('th').filter({hasText: /^Valor\s*$/i}).click(),
  ]);
  const first = (await sortedResponse.json()).items[0];
  await page.locator('tbody tr').first().getByTitle('Clique para alterar categoria').click();
  const [categoryResponse] = await Promise.all([
   page.waitForResponse(item => item.url().endsWith('/transactions/' + first.id) && item.request().method() === 'PUT'),
   page.locator('tbody tr').first().locator('select').selectOption(category.id),
  ]);
  assert.equal(categoryResponse.status(), 200);
  assert.equal((await getJSON('/api/v1/transactions/' + first.id)).category_id, category.id);
  checks.push('sorting and inline category classification read back');
  await page.locator('tbody tr').first().getByTitle('Editar transação').click();
  await page.locator('form input[type=text]').first().fill('SYNTHETIC BROWSER EDIT');
  const [editResponse] = await Promise.all([
   page.waitForResponse(item => item.url().endsWith('/transactions/' + first.id) && item.request().method() === 'PUT'),
   page.getByRole('button', {name: 'Salvar Alterações', exact: true}).click(),
  ]);
  assert.equal(editResponse.status(), 200);
  assert.equal((await getJSON('/api/v1/transactions/' + first.id)).description, 'SYNTHETIC BROWSER EDIT');
  await page.locator('tbody tr').first().getByTitle('Excluir transação').click();
  const [deleteResponse] = await Promise.all([
   page.waitForResponse(item => item.url().endsWith('/transactions/' + first.id) && item.request().method() === 'DELETE'),
   page.getByRole('button', {name: 'Confirmar Exclusão', exact: true}).click(),
  ]);
  assert.equal(deleteResponse.status(), 204);
  assert.equal((await page.request.get(baseURL + '/api/v1/transactions/' + first.id)).status(), 404);
  created.delete(first.id);
  await page.getByText(/\(99 registros no total\)/).waitFor();
  checks.push('edit and single deletion through UI, read back by API');
  await page.locator('tbody tr input[type=checkbox]').nth(0).check();
  await page.locator('tbody tr input[type=checkbox]').nth(1).check();
  await page.getByRole('button', {name: 'Excluir', exact: true}).click();
  const [batchResponse] = await Promise.all([
   page.waitForResponse(item => item.url().endsWith('/transactions/batch-delete') && item.request().method() === 'POST'),
   page.getByRole('button', {name: 'Confirmar Exclusão', exact: true}).click(),
  ]);
  assert.equal(batchResponse.status(), 204);
  const remaining = await getJSON('/api/v1/transactions?limit=200');
  assert.equal(remaining.pagination.total, 97);
  created.clear();
  remaining.items.forEach(item => created.add(item.id));
  await page.getByText(/\(97 registros no total\)/).waitFor();
  checks.push('selection and bulk deletion through UI');
  if (process.env.SMOKE_SCREENSHOT) await page.screenshot({path: process.env.SMOKE_SCREENSHOT, fullPage: true});
  await page.setViewportSize({width: 390, height: 844});
  assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1), 'mobile layout must not overflow the viewport');
  checks.push('mobile viewport without document overflow');
  assert.deepEqual(errors, []);
  checks.push('no JavaScript runtime errors');
 } finally {
  try {
   await cleanup();
  } finally {
   await browser.close();
  }
 }
 console.log(JSON.stringify({status: 'PASS', checks}, null, 2));
})().catch(error => { console.error(error); process.exitCode = 1; });
