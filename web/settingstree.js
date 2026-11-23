// The Settings sidebar tree: pure, no imports (Node-tested). Section numbers are indexes in page order.

// sections: { title, topic, topicTitle, router?, routerTitle? } in page order. A topic's sections come before its
// router blocks, so a router's sections stay together.
export function topicTree(sections) {
  const topics = [];
  sections.forEach((s, i) => {
    let t = topics.find((x) => x.id === s.topic);
    if (!t) topics.push((t = { id: s.topic, title: s.topicTitle, first: i, sections: [], routers: [] }));
    if (!s.router) return t.sections.push(i);
    let r = t.routers.find((x) => x.id === s.router);
    if (!r) t.routers.push((r = { id: s.router, title: s.routerTitle, first: i, sections: [] }));
    r.sections.push(i);
  });
  return topics;
}

export function topicOf(tree, i) {
  for (const [k, t] of tree.entries()) {
    if (t.sections.includes(i)) return { topic: k, router: null };
    const r = t.routers.find((x) => x.sections.includes(i));
    if (r) return { topic: k, router: r.id };
  }
  return { topic: -1, router: null };
}
