import { test } from 'node:test';
import assert from 'node:assert/strict';
import { stateText, fromText, countLabel, itemsText, acceptPick, acceptReq, sendBody, toastText } from '../inboxrules.js';

test('Sent list states', () => {
  assert.equal(stateText({ state: 'waiting', count: 5 }), 'Waiting');
  assert.equal(stateText({ state: 'accepted', count: 5, accepted: 5 }), 'Accepted');
  assert.equal(stateText({ state: 'accepted', count: 5, accepted: 4 }), 'Accepted 4 of 5');
  assert.equal(stateText({ state: 'accepted', count: 5 }), 'Accepted 0 of 5');
  assert.equal(stateText({ state: 'declined', count: 1 }), 'Declined');
  assert.equal(stateText({ state: 'expired', count: 1 }), 'Expired');
  assert.equal(stateText({ state: 'withdrawn', count: 1 }), 'Withdrawn');
});

test('a removed sender', () => {
  assert.equal(fromText({ from: 'Ana' }), 'From Ana');
  assert.equal(fromText({ from: '' }), 'From a removed user');
});

test('the button label says the count', () => {
  assert.equal(countLabel(0), 'Inbox');
  assert.equal(countLabel(1), 'Inbox, 1 card waiting');
  assert.equal(countLabel(3), 'Inbox, 3 cards waiting');
});

test('items of a card', () => {
  const items = [
    { category: 'presets', key: 'p1', name: 'Show' },
    { category: 'presets', key: 'p2', name: 'Desk' },
    { category: 'panels', key: 'x', name: 'Studio' },
  ];
  assert.equal(itemsText(items), '2 presets, 1 panel');
  assert.equal(itemsText([{ category: 'filterSets', key: 'Cams', name: 'Cams' }]), '1 filter set');
  assert.equal(toastText({ from: 'Ana', items }), 'Ana sent you 2 presets, 1 panel.');
  assert.equal(toastText({ from: '', count: 2 }), 'A removed user sent you 2 items.');
});

// The plan as inboxReview answers it (importPlan): a clash defaults to Rename.
const plan = () => ({
  categories: {
    presets: {
      items: [
        { key: 'p1', name: 'Show', clash: 'Show', choice: 'rename', as: 'Show 2' },
        { key: 'p2', name: 'Desk', note: 'Outside your Rights' },
      ],
    },
    filterSets: { items: [{ key: 'Cams', name: 'Cams' }] },
    panels: { refused: 'the router names are not read yet', items: [{ key: 'x', name: 'Studio' }] },
  },
});

test('Accept starts with every item ticked and the clash choices', () => {
  const pick = acceptPick(plan());
  assert.deepEqual([...pick.items.presets], ['p1', 'p2']);
  assert.deepEqual([...pick.items.filterSets], ['Cams']);
  assert.deepEqual([...pick.items.panels], []);
  assert.deepEqual(pick.choices.presets, { p1: 'rename' });
});

// An empty list is sent, never left out: the server takes a missing category as every item (cardPlanner.picked).
test('an unticked category goes as an empty list', () => {
  const pick = acceptPick(plan());
  pick.items.presets.delete('p1');
  pick.items.filterSets.clear();
  assert.deepEqual(acceptReq(pick), { pick: { presets: ['p2'], filterSets: [], panels: [] }, choices: {} });
  pick.items.presets.add('p1');
  pick.choices.presets.p1 = 'skip';
  assert.deepEqual(acceptReq(pick).choices, { presets: { p1: 'skip' } });
});

test('send body: Me and Everyone by id or name, This browser whole', () => {
  const local = { scope: 'local', id: 'l1', name: 'Desk', pinned: true, items: [{ src: 3, dsts: [7], levels: [], srcName: 'CAM 3', dstNames: {} }] };
  const body = sendBody('r1', ['Bo', 'Cy'], {
    presets: [{ scope: 'user', id: 'p1', name: 'Show' }, local],
    filterSets: [
      { scope: 'global', name: 'Cams' },
      { scope: 'local', name: 'Mine', f: { src: 'CAM' } },
    ],
    panels: [{ id: 'x' }],
  });
  assert.deepEqual(body, {
    to: ['Bo', 'Cy'],
    presets: [
      { scope: 'user', id: 'p1' },
      {
        scope: 'local',
        preset: {
          id: 'l1',
          name: 'Desk',
          pinned: true,
          items: [{ src: { router: 'r1', n: 3, name: 'CAM 3' }, dsts: [{ router: 'r1', n: 7 }], levels: [] }],
        },
      },
    ],
    filterSets: [
      { scope: 'global', name: 'Cams' },
      { scope: 'local', set: { name: 'Mine', src: 'CAM' } },
    ],
    panels: ['x'],
  });
});
