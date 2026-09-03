import type { Metadata } from 'next';
import './globals.scss';
import { Providers, GlobalUI } from '@/components/providers/Providers';

export const metadata: Metadata = {
  title: 'Đông Đô Partners - Hệ Thống Chăm Sóc Khách Hàng Thông Minh',
  description:
    'Tư vấn Hàng hóa phái sinh, DDP Invest, nạp rút tiền và quản trị rủi ro trực tuyến 24/7',
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="vi">
      <body>
        <Providers>
          {children}
          <GlobalUI />
        </Providers>
      </body>
    </html>
  );
}
