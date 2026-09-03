// Allow side-effect CSS/SCSS imports used by global styles and SCSS modules
declare module '*.css' {
  const content: Record<string, string>;
  export default content;
  export = content;
}

declare module '*.scss' {
  const content: Record<string, string>;
  export default content;
  export = content;
}

declare module '*.module.scss' {
  const content: Record<string, string>;
  export default content;
  export = content;
}
