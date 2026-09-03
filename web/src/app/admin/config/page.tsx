'use client';

import React, { useState } from 'react';
import {
  useConfig,
  useUpdateConfig,
} from '@/lib/hooks/useApi';
import { ConfigForm } from '@/components/forms/ConfigForm';
import type { SystemConfig } from '@/lib/types';
import {
  Settings,
  RefreshCw,
  Check,
  Save,
  AlertCircle,
  CheckCircle,
  Bot,
  Cpu,
  Sparkles,
} from 'lucide-react';

export default function ConfigPage() {
  const [successMessage, setSuccessMessage] = useState<string | null>(null);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);

  const { data: configData, isLoading, refetch } = useConfig();
  const updateConfigMutation = useUpdateConfig();

  const handleSubmit = async (data: { system_prompt: string; llm_model: string; temperature: number }) => {
    setErrorMessage(null);
    setSuccessMessage(null);

    try {
      await updateConfigMutation.mutateAsync(data as SystemConfig);
      setSuccessMessage('Cấu hình đã được lưu thành công!');
      setTimeout(() => setSuccessMessage(null), 4000);
      refetch();
    } catch (err: any) {
      setErrorMessage(err.message || 'Lỗi khi lưu cấu hình');
    }
  };

  const defaultValues = {
    system_prompt: configData?.system_prompt || '',
    llm_model: configData?.llm_model || 'claude-haiku-4-5-20251001',
    temperature: configData?.temperature ?? 0.1,
  };

  return (
    <div className="p-6 space-y-6 bg-[#0A0F1D] min-h-full">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center space-x-3">
          <div className="w-12 h-12 rounded-xl bg-slate-500/20 border border-slate-500/30 flex items-center justify-center text-slate-400">
            <Settings className="w-6 h-6" />
          </div>
          <div>
            <h1 className="text-xl font-bold text-white">Cấu Hình LLM Studio</h1>
            <p className="text-sm text-slate-400">System Prompt, Model và Temperature cho AI</p>
          </div>
        </div>
        <button
          onClick={() => refetch()}
          className="px-4 py-2 rounded-xl bg-slate-800 hover:bg-slate-700 text-slate-300 text-sm font-semibold flex items-center space-x-2 cursor-pointer"
        >
          <RefreshCw className="w-4 h-4" />
          <span>Làm mới</span>
        </button>
      </div>

      {/* Info Cards */}
      <div className="grid grid-cols-3 gap-4">
        <div className="bg-[#0D1527] border border-slate-800/80 rounded-2xl p-4">
          <div className="flex items-center space-x-3">
            <div className="w-10 h-10 rounded-xl bg-purple-500/20 flex items-center justify-center text-purple-400">
              <Bot className="w-5 h-5" />
            </div>
            <div>
              <p className="text-xs text-slate-400 font-medium">Model</p>
              <p className="text-sm font-semibold text-white mt-0.5">
                {configData?.llm_model?.split('-')[0] || 'Claude'} {configData?.llm_model?.includes('haiku') ? 'Haiku' : configData?.llm_model?.includes('sonnet') ? 'Sonnet' : 'Opus'}
              </p>
            </div>
          </div>
        </div>
        <div className="bg-[#0D1527] border border-slate-800/80 rounded-2xl p-4">
          <div className="flex items-center space-x-3">
            <div className="w-10 h-10 rounded-xl bg-blue-500/20 flex items-center justify-center text-blue-400">
              <Cpu className="w-5 h-5" />
            </div>
            <div>
              <p className="text-xs text-slate-400 font-medium">Temperature</p>
              <p className="text-sm font-semibold text-white mt-0.5">
                {configData?.temperature?.toFixed(1) || '0.1'}
              </p>
            </div>
          </div>
        </div>
        <div className="bg-[#0D1527] border border-slate-800/80 rounded-2xl p-4">
          <div className="flex items-center space-x-3">
            <div className="w-10 h-10 rounded-xl bg-emerald-500/20 flex items-center justify-center text-emerald-400">
              <Sparkles className="w-5 h-5" />
            </div>
            <div>
              <p className="text-xs text-slate-400 font-medium">Prompt Length</p>
              <p className="text-sm font-semibold text-white mt-0.5">
                {configData?.system_prompt?.length || 0} chars
              </p>
            </div>
          </div>
        </div>
      </div>

      {/* Config Form */}
      {isLoading ? (
        <div className="bg-[#0D1527] border border-slate-800/80 rounded-2xl p-12">
          <div className="flex flex-col items-center justify-center text-slate-400">
            <RefreshCw className="w-8 h-8 animate-spin mb-3" />
            <span className="text-sm">Đang tải cấu hình...</span>
          </div>
        </div>
      ) : (
        <div className="bg-[#0D1527] border border-slate-800/80 rounded-2xl p-6">
          {/* Success Message */}
          {successMessage && (
            <div className="mb-4 p-3 rounded-xl bg-emerald-500/10 border border-emerald-500/30 flex items-center space-x-2 text-emerald-300">
              <CheckCircle className="w-5 h-5 shrink-0" />
              <span className="text-sm">{successMessage}</span>
            </div>
          )}

          {/* Error Message */}
          {errorMessage && (
            <div className="mb-4 p-3 rounded-xl bg-rose-500/10 border border-rose-500/30 flex items-center space-x-2 text-rose-300">
              <AlertCircle className="w-5 h-5 shrink-0" />
              <span className="text-sm">{errorMessage}</span>
            </div>
          )}

          <ConfigForm
            defaultValues={defaultValues}
            onSubmit={handleSubmit}
            isLoading={updateConfigMutation.isPending}
            successMessage={successMessage || undefined}
          />
        </div>
      )}

      {/* Tips Section */}
      <div className="bg-indigo-500/10 border border-indigo-500/20 rounded-2xl p-4">
        <h3 className="text-sm font-semibold text-indigo-400 mb-2 flex items-center space-x-2">
          <Sparkles className="w-4 h-4" />
          <span>Mẹo cấu hình</span>
        </h3>
        <ul className="text-xs text-indigo-300/80 space-y-1.5 ml-6">
          <li>• <strong>Claude Haiku:</strong> Nhanh, rẻ, phù hợp cho hầu hết tác vụ CSKH thông thường</li>
          <li>• <strong>Claude Sonnet:</strong> Cân bằng giữa tốc độ và chất lượng</li>
          <li>• <strong>Claude Opus:</strong> Mạnh nhất, dùng cho các truy vấn phức tạp, chi phí cao hơn</li>
          <li>• <strong>Temperature thấp (0.1-0.3):</strong> Câu trả lời nhất quán, chính xác hơn</li>
          <li>• <strong>Temperature cao (0.7-1.0):</strong> Sáng tạo hơn, có thể ít nhất quán hơn</li>
        </ul>
      </div>
    </div>
  );
}
