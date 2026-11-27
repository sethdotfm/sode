import test from 'node:test';
import assert from 'node:assert/strict';
import { topicTree, topicOf, routerEntries } from '../settingstree.js';

const secs = [
  { title: 'This browser', topic: 'preferences', topicTitle: 'Preferences' },
  { title: 'My user', topic: 'preferences', topicTitle: 'Preferences' },
  { title: 'Branding', topic: 'look', topicTitle: 'Look and feel' },
  { title: 'Routers', topic: 'routers', topicTitle: 'Routers' },
  { title: 'Tie lines', topic: 'routers', topicTitle: 'Routers' },
  { title: 'Protects', topic: 'routers', topicTitle: 'Routers' },
  { title: 'Proxy', topic: 'server', topicTitle: 'Server' },
];

test('topics in page order, each with its sections', () => {
  assert.deepEqual(topicTree(secs), [
    { id: 'preferences', title: 'Preferences', first: 0, sections: [0, 1] },
    { id: 'look', title: 'Look and feel', first: 2, sections: [2] },
    { id: 'routers', title: 'Routers', first: 3, sections: [3, 4, 5] },
    { id: 'server', title: 'Server', first: 6, sections: [6] },
  ]);
});

test('topicOf names the topic of a section', () => {
  const tree = topicTree(secs);
  assert.equal(topicOf(tree, 1), 0);
  assert.equal(topicOf(tree, 5), 2);
  assert.equal(topicOf(tree, 6), 3);
  assert.equal(topicOf(tree, -1), -1);
});

test('Routers > Routers has an entry for each router only with two or more', () => {
  assert.deepEqual(routerEntries([{ uid: 'a', name: 'Studio' }], 'a'), []);
  assert.deepEqual(
    routerEntries(
      [
        { uid: 'a', name: 'A Studio' },
        { uid: 'b', name: 'B Stage 2' },
      ],
      'b',
    ),
    [
      { uid: 'a', title: 'A Studio', current: false },
      { uid: 'b', title: 'B Stage 2', current: true },
    ],
  );
});
