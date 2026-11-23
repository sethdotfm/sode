// prev: the label of the page's first hello. Another label later means someone upgraded the server under an open page.
export const buildChanged = (prev, next) => !!prev && !!next && prev !== next;
