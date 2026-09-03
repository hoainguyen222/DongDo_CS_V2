'use client';

import React, { Suspense, useState, useEffect } from 'react';
import { useSearchParams, useRouter } from 'next/navigation';
import { ShieldCheck, AlertCircle, CheckCircle2, ExternalLink } from 'lucide-react';
import { api } from '@/lib/api';

function SetupPageInner() {
  const searchParams = useSearchParams();
  const router = useRouter();
  const adminPath = (process.env.NEXT_PUBLIC_ADMIN_PATH as string | undefined) || '/admin';

  const [status, setStatus] = useState<{ needs_setup: boolean; is_enabled: boolean } | null>(null);
  const [form, setForm] = useState({ username: '', password: '', fullName: '' });
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState('');
  const [success, setSuccess] = useState(false);

  useEffect(() => {
    fetch(`${process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8080'}/api/bootstrap/status`)
      .then(r => r.json())
      .then(d => setStatus(d))
      .catch(() => setStatus({ needs_setup: true, is_enabled: true }));
  }, []);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsLoading(true);
    setError('');
    try {
      const apiBase = (process.env.NEXT_PUBLIC_API_URL as string | undefined) || 'http://localhost:8080';
      const res = await fetch(`${apiBase}/api/bootstrap/install`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(form),
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error || 'Cài đặt thất bại');
      setSuccess(true);
    } catch (err: any) {
      setError(err.message);
    } finally {
      setIsLoading(false);
    }
  };

  if (!status) {
    return (
      <div className="min-h-screen bg-[#0A0F1D] flex items-center justify-center">
        <div className="w-10 h-10 border-4 border-[#B32D38] border-t-transparent rounded-full animate-spin" />
      </div>
    );
  }

  // Not needed / not enabled
  if (!status.needs_setup) {
    return (
      <div className="min-h-screen bg-[#0A0F1D] flex items-center justify-center p-4">
        <div className="max-w-md text-center glass-panel-brand p-8 rounded-2xl">
          <CheckCircle2 className="w-16 h-16 mx-auto text-emerald-400 mb-4" />
          <h2 className="text-xl font-bold text-white mb-2">Hệ thống đã được thiết lập</h2>
          <p className="text-sm text-slate-400 mb-6">Tài khoản quản trị đã có sẵn. Vui lòng đăng nhập.</p>
          <a
            href={`${adminPath}/login`}
            className="inline-flex items-center space-x-2 px-6 py-3 rounded-xl btn-brand-primary text-sm font-semibold"
          >
            <span>Đăng nhập</span>
          </a>
        </div>
      </div>
    );
  }

  if (!status.is_enabled) {
    return (
      <div className="min-h-screen bg-[#0A0F1D] flex items-center justify-center p-4">
        <div className="max-w-md text-center glass-panel-brand p-8 rounded-2xl">
          <AlertCircle className="w-16 h-16 mx-auto text-amber-400 mb-4" />
          <h2 className="text-xl font-bold text-white mb-2">Bootstrap bị vô hiệu hóa</h2>
          <p className="text-sm text-slate-400 mb-6">
            Để thiết lập tài khoản Owner đầu tiên, vui lòng thêm biến môi trường <code className="bg-slate-800 px-1.5 py-0.5 rounded text-xs">ENABLE_BOOTSTRAP=true</code> vào backend và restart server.
          </p>
          <a href="/" className="text-xs text-slate-400 hover:text-white transition">← Quay về trang chủ</a>
        </div>
      </div>
    );
  }

  if (success) {
    return (
      <div className="min-h-screen bg-[#0A0F1D] flex items-center justify-center p-4">
        <div className="max-w-md text-center glass-panel-brand p-8 rounded-2xl">
          <CheckCircle2 className="w-16 h-16 mx-auto text-emerald-400 mb-4" />
          <h2 className="text-xl font-bold text-white mb-2">Thiết lập thành công!</h2>
          <p className="text-sm text-slate-400 mb-2">
            Tài khoản <strong className="text-white">{form.username}</strong> đã được tạo với vai trò Owner.
          </p>
          <div className="bg-amber-500/10 border border-amber-500/30 rounded-xl p-3 text-xs text-amber-300 mb-6 text-left">
            <strong>⚠️ Quan trọng:</strong> Vui lòng tắt <code className="bg-slate-800 px-1 py-0.5 rounded">ENABLE_BOOTSTRAP=true</code> trong file <code className="bg-slate-800 px-1 py-0.5 rounded">.env</code> và restart server để bảo mật hệ thống.
          </div>
          <a
            href={`${adminPath}/login`}
            className="inline-flex items-center space-x-2 px-6 py-3 rounded-xl btn-brand-primary text-sm font-semibold"
          >
            <span>Đăng nhập ngay</span>
            <ExternalLink className="w-4 h-4" />
          </a>
        </div>
      </div>
    );
  }

  return (
    <div className="min-h-screen bg-[#0A0F1D] flex items-center justify-center p-4">
      <div className="w-full max-w-lg glass-panel-brand p-8 rounded-2xl relative z-10 shadow-2xl">
        <div className="text-center mb-6">
          <div className="inline-flex items-center justify-center p-3 rounded-2xl bg-[#1C2D56] border border-[#95252E]/40 mb-4 shadow-lg">
            <img
              src="/logo/Logo Dọc_Trắng.svg"
              alt="Đông Đô Partners"
              className="h-12 w-auto object-contain"
              onError={(e) => { e.currentTarget.style.display = 'none'; }}
            />
          </div>
          <h2 className="text-xl font-bold text-white tracking-tight">Thiết lập Hệ thống CSKH</h2>
          <p className="text-xs text-slate-400 mt-1">Tạo tài khoản Owner đầu tiên để quản trị hệ thống</p>
        </div>

        <div className="bg-emerald-500/10 border border-emerald-500/30 rounded-xl p-3 text-xs text-emerald-300 mb-6">
          <ShieldCheck className="w-4 h-4 inline mr-1.5 mb-0.5" />
          Đây là bước thiết lập ban đầu. Sau khi hoàn tất, hệ thống sẽ yêu cầu đăng nhập để truy cập.
        </div>

        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="block text-xs font-semibold text-slate-300 uppercase tracking-wider mb-2">Họ và tên</label>
            <input
              type="text"
              value={form.fullName}
              onChange={e => setForm(f => ({ ...f, fullName: e.target.value }))}
              placeholder="Nguyễn Văn A"
              required
              className="w-full px-4 py-3 rounded-xl glass-input text-sm"
            />
          </div>
          <div>
            <label className="block text-xs font-semibold text-slate-300 uppercase tracking-wider mb-2">Tên đăng nhập</label>
            <input
              type="text"
              value={form.username}
              onChange={e => setForm(f => ({ ...f, username: e.target.value.toLowerCase().replace(/\s/g, '') }))}
              placeholder="admin hoặc owner"
              required
              minLength={3}
              maxLength={50}
              className="w-full px-4 py-3 rounded-xl glass-input text-sm"
            />
          </div>
          <div>
            <label className="block text-xs font-semibold text-slate-300 uppercase tracking-wider mb-2">Mật khẩu</label>
            <input
              type="password"
              value={form.password}
              onChange={e => setForm(f => ({ ...f, password: e.target.value }))}
              placeholder="Tối thiểu 8 ký tự"
              required
              minLength={8}
              maxLength={50}
              className="w-full px-4 py-3 rounded-xl glass-input text-sm"
            />
          </div>

          {error && (
            <div className="p-3 rounded-xl bg-rose-500/10 border border-rose-500/30 text-rose-300 text-xs flex items-center space-x-2">
              <AlertCircle className="w-4 h-4 shrink-0" />
              <span>{error}</span>
            </div>
          )}

          <button
            type="submit"
            disabled={isLoading || !form.username || !form.password || !form.fullName}
            className="w-full py-3.5 px-4 rounded-xl btn-brand-primary flex items-center justify-center space-x-2 text-sm font-semibold mt-6 cursor-pointer disabled:opacity-50 transition shadow-lg"
          >
            <ShieldCheck className="w-4 h-4" />
            <span>{isLoading ? 'Đang thiết lập...' : 'Tạo Tài khoản Owner'}</span>
          </button>
        </form>

        <div className="mt-4 text-center">
          <a href="/" className="text-xs text-slate-500 hover:text-slate-300 transition">← Quay về trang chủ</a>
        </div>
      </div>
    </div>
  );
}

export default function AdminSetupPage() {
  return (
    <Suspense fallback={<div className="min-h-screen bg-[#0A0F1D]" />}>
      <SetupPageInner />
    </Suspense>
  );
}
