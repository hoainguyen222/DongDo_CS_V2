// Allow side-effect CSS imports used by PartnerStyles.css and global styles
declare module '*.css' {
  const content: Record<string, string>;
  export default content;
  export = content;
}
