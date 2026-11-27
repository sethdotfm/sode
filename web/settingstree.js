// The Settings sidebar tree: pure, no imports (Node-tested). Section numbers are indexes in page order.

// sections: { title, topic, topicTitle } in page order.
export function topicTree(sections) {
  const topics = [];
  sections.forEach((s, i) => {
    let t = topics.find((x) => x.id === s.topic);
    if (!t) topics.push((t = { id: s.topic, title: s.topicTitle, first: i, sections: [] }));
    t.sections.push(i);
  });
  return topics;
}

export function topicOf(tree, i) {
  return tree.findIndex((t) => t.sections.includes(i));
}

// routerEntries: the entries under Routers > Routers, one for each router, only with two or more. routers: [{uid, name}]
// in list order; selected: the UID in the detail view.
export const routerEntries = (routers, selected) =>
  routers.length > 1 ? routers.map((r) => ({ uid: r.uid, title: r.name, current: r.uid === selected })) : [];
