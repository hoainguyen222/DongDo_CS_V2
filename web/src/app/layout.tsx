import type { Metadata } from 'next';
import './globals.css';

export const metadata: Metadata = {
  title: 'Đông Đô Partners - Hệ Thống Chăm Sóc Khách Hàng Thông Minh',
  description: 'Tư vấn Hàng hóa phái sinh, DDP Invest, nạp rút tiền và quản trị rủi ro trực tuyến 24/7',
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="vi">
      <body className="antialiased bg-[#0b0f19] text-slate-100 min-h-screen">
        {children}
      </body>
    </html>
  );
}
