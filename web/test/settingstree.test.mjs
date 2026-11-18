import test from 'node:test';
import assert from 'node:assert/strict';
import { topicTree, topicOf } from '../settingstree.js';

const secs = [
  { title: 'This browser', topic: 'preferences', topicTitle: 'Preferences' },
  { title: 'My user', topic: 'preferences', topicTitle: 'Preferences' },
  { title: 'Branding', topic: 'look', topicTitle: 'Look and feel' },
  { title: 'Routers', topic: 'routers', topicTitle: 'Routers' },
  { title: 'Router & matrix', topic: 'routers', topicTitle: 'Routers', router: '1', routerTitle: 'Ultrix' },
  { title: 'Takes', topic: 'routers', topicTitle: 'Routers', router: '1', routerTitle: 'Ultrix' },
  { title: 'Proxy', topic: 'server', topicTitle: 'Server' },
];

test('topics in page order, each with its sections; a router groups its own', () => {
  assert.deepEqual(topicTree(secs), [
    { id: 'preferences', title: 'Preferences', first: 0, sections: [0, 1], routers: [] },
    { id: 'look', title: 'Look and feel', first: 2, sections: [2], routers: [] },
    { id: 'routers', title: 'Routers', first: 3, sections: [3], routers: [{ id: '1', title: 'Ultrix', first: 4, sections: [4, 5] }] },
    { id: 'server', title: 'Server', first: 6, sections: [6], routers: [] },
  ]);
});

test('topicOf names the topic and router of a section', () => {
  const tree = topicTree(secs);
  assert.deepEqual(topicOf(tree, 1), { topic: 0, router: null });
  assert.deepEqual(topicOf(tree, 3), { topic: 2, router: null });
  assert.deepEqual(topicOf(tree, 5), { topic: 2, router: '1' });
  assert.deepEqual(topicOf(tree, 6), { topic: 3, router: null });
  assert.deepEqual(topicOf(tree, -1), { topic: -1, router: null });
});
