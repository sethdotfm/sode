// prev is the build label of the page's first hello, so it names the build that served this page.
// A later hello with another label means the server was upgraded under an open page.
export const buildChanged = (prev, next) => !!prev && !!next && prev !== next;
