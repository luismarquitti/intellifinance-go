// Deliberately tiny Playwright boundary for operational fault-injection tests.
const fs = require('node:fs');
const scenario = process.env.FAULT;
const state = {items: scenario === 'preexisting' ? [{id: 'old', account_id: 'checking'}] : [], deleted: [], closed: false, committed: false};
const save = () => fs.writeFileSync(process.env.STATE_FILE, JSON.stringify(state));
save();
let queryFailed = false;
const response = (body, status = 200) => ({status: () => status, json: async () => body});
const get = async url => {
 const u = new URL(url);
 if (u.pathname === '/api/v1/session') return response({environment: 'development', user_name: 'Synthetic Admin'});
 if (u.pathname === '/api/v1/accounts') return response({items: [{id: 'checking', type: 'CHECKING'}]});
 if (u.pathname === '/api/v1/categories') return response({items: [{id: 'category', type: 'EXPENSE'}]});
 if (u.pathname === '/api/v1/transactions') {
  if (state.committed && (scenario === 'offline' || (scenario === 'query-transient' && !queryFailed))) {
   queryFailed = true;
   throw new Error('query unavailable');
  }
  const account = u.searchParams.get('account_id');
  const items = state.items.filter(tx => !account || tx.account_id === account);
  return response({items, pagination: {total: items.length}});
 }
 throw new Error('unexpected fake GET ' + url);
};
const locator = name => ({
 waitFor: async () => {},
 innerText: async () => '(0 registros no total)',
 selectOption: async () => {},
 setInputFiles: async () => {},
 click: async () => {
  if (name === 'Importar Extrato') {
   state.committed = true;
   state.items = [{id: 'new', account_id: 'checking'}, {id: 'foreign', account_id: 'other'}];
   save();
  }
  if (name === 'Concluir') throw new Error('stop after upload for cleanup test');
 },
});
const page = {
 on: () => {}, goto: async () => {},
 getByText: name => locator(name), getByRole: (_, opts) => locator(opts.name), locator,
 waitForResponse: async () => {
  await new Promise(resolve => setImmediate(resolve));
  if (scenario === 'lost-response' || scenario === 'offline') throw new Error('response lost after commit');
  return response({inserted: 100, duplicates: 0});
 },
 request: {get, post: async (url, {data}) => {
  if (!url.endsWith('/batch-delete')) throw new Error('unexpected fake POST');
  state.deleted.push(...data.ids);
  if (scenario !== 'unconfirmed-delete') state.items = state.items.filter(tx => !data.ids.includes(tx.id));
  save();
  return response(null, 204);
 }},
};
module.exports = {chromium: {launch: async () => ({newPage: async () => page, close: async () => {state.closed = true; save();}})}};
