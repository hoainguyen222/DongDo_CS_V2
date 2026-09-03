// PostCSS chỉ cần Autoprefixer + SCSS cho SCSS Modules (từ Next.js built-in)
// Tailwind đã được gỡ bỏ. Project chuyển sang SCSS Modules (file .module.scss).
module.exports = {
  plugins: {
    autoprefixer: {},
  },
};
