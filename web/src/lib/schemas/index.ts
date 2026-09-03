/**
 * Zod Schemas — Runtime validation cho form inputs
 * Dùng chung với React Hook Form
 */

import { z } from 'zod';

// ============================================================
// ─── AUTH ──────────────────────────────────────────────────

export const loginSchema = z.object({
  username: z
    .string()
    .min(1, 'Tên đăng nhập không được để trống')
    .max(50, 'Tên đăng nhập tối đa 50 ký tự'),
  password: z
    .string()
    .min(6, 'Mật khẩu tối thiểu 6 ký tự')
    .max(50, 'Mật khẩu tối đa 50 ký tự'),
});

export type LoginFormData = z.infer<typeof loginSchema>;

// ============================================================
// ─── GUEST REGISTRATION ─────────────────────────────────────

export const guestRegisterSchema = z.object({
  displayName: z
    .string()
    .min(1, 'Họ tên không được để trống')
    .max(100, 'Họ tên tối đa 100 ký tự')
    .regex(/^[A-Za-zÀ-ỹ\s]+$/, 'Họ tên chỉ chứa chữ cái và dấu cách'),
  phone: z
    .string()
    .optional()
    .refine(
      (val) => !val || /^[\d\s+()-]{8,15}$/.test(val),
      'Số điện thoại không hợp lệ'
    ),
});

export type GuestRegisterFormData = z.infer<typeof guestRegisterSchema>;

// ============================================================
// ─── SYSTEM CONFIG ─────────────────────────────────────────

export const configSchema = z.object({
  system_prompt: z.string().min(1, 'System prompt không được trống'),
  llm_model: z.string().min(1, 'Vui lòng chọn model LLM'),
  temperature: z
    .number()
    .min(0, 'Temperature tối thiểu là 0')
    .max(2, 'Temperature tối đa là 2'),
});

export type ConfigFormData = z.infer<typeof configSchema>;

// ============================================================
// ─── LEARNING ITEM EDIT ────────────────────────────────────

export const learningItemSchema = z.object({
  question: z
    .string()
    .min(5, 'Câu hỏi tối thiểu 5 ký tự')
    .max(500, 'Câu hỏi tối đa 500 ký tự'),
  answer: z
    .string()
    .min(5, 'Câu trả lời tối thiểu 5 ký tự')
    .max(2000, 'Câu trả lời tối đa 2000 ký tự'),
});

export type LearningItemFormData = z.infer<typeof learningItemSchema>;

// ============================================================
// ─── QA PAIR (Resolve Case Modal) ─────────────────────────

export const qaPairSchema = z.object({
  question: z
    .string()
    .min(1, 'Câu hỏi không được trống nếu bật dạy AI')
    .max(500, 'Câu hỏi tối đa 500 ký tự'),
  answer: z
    .string()
    .min(1, 'Câu trả lời không được trống nếu bật dạy AI')
    .max(2000, 'Câu trả lời tối đa 2000 ký tự'),
});

export type QAPairFormData = z.infer<typeof qaPairSchema>;

export const qaPairsSchema = z.object({
  qaPairs: z.array(qaPairSchema),
});

export type QAPairsFormData = z.infer<typeof qaPairsSchema>;

// ============================================================
// ─── CUSTOMER UPDATE ────────────────────────────────────────

export const customerUpdateSchema = z.object({
  displayName: z
    .string()
    .min(1, 'Tên khách hàng không được để trống')
    .max(100, 'Tên tối đa 100 ký tự'),
  phone: z
    .string()
    .optional()
    .refine(
      (val) => !val || /^[\d\s+()-]{8,15}$/.test(val),
      'Số điện thoại không hợp lệ'
    ),
});

export type CustomerUpdateFormData = z.infer<typeof customerUpdateSchema>;

// ============================================================
// ─── USER ACCOUNT ───────────────────────────────────────────

export const userCreateSchema = z.object({
  fullName: z
    .string()
    .min(1, 'Họ tên không được để trống')
    .max(100, 'Họ tên tối đa 100 ký tự'),
  email: z
    .string()
    .min(1, 'Email/tài khoản không được để trống')
    .max(50, 'Tài khoản tối đa 50 ký tự'),
  role: z.enum(['owner', 'admin', 'leader', 'cskh'], {
    errorMap: () => ({ message: 'Vui lòng chọn vai trò hợp lệ' }),
  }),
  password: z
    .string()
    .min(6, 'Mật khẩu tối thiểu 6 ký tự')
    .max(50, 'Mật khẩu tối đa 50 ký tự')
    .optional(),
});

export type UserCreateFormData = z.infer<typeof userCreateSchema>;

export const userUpdateSchema = z.object({
  fullName: z
    .string()
    .min(1, 'Họ tên không được để trống')
    .max(100, 'Họ tên tối đa 100 ký tự'),
  role: z.enum(['owner', 'admin', 'leader', 'cskh'], {
    errorMap: () => ({ message: 'Vui lòng chọn vai trò hợp lệ' }),
  }),
  isActive: z.boolean(),
  password: z
    .string()
    .max(50, 'Mật khẩu tối đa 50 ký tự')
    .optional(),
});

export type UserUpdateFormData = z.infer<typeof userUpdateSchema>;

// ============================================================
// ─── ROLE PERMISSION ────────────────────────────────────────

export const rolePermissionSchema = z.object({
  permission_level: z.enum(['act', 'view', 'none'], {
    errorMap: () => ({ message: 'Vui lòng chọn quyền hợp lệ: act, view, hoặc none' }),
  }),
});

export type RolePermissionFormData = z.infer<typeof rolePermissionSchema>;

// ============================================================
// ─── SEND CS MESSAGE ────────────────────────────────────────

export const csMessageSchema = z.object({
  content: z
    .string()
    .min(1, 'Tin nhắn không được trống')
    .max(5000, 'Tin nhắn tối đa 5000 ký tự'),
});

export type CSMessageFormData = z.infer<typeof csMessageSchema>;

// ============================================================
// ─── UPLOAD DOCUMENT ────────────────────────────────────────

export const documentUploadSchema = z.object({
  file: z
    .instanceof(File, { message: 'Vui lòng chọn file' })
    .refine(
      (file) =>
        [
          'application/vnd.openxmlformats-officedocument.wordprocessingml.document',
          'application/msword',
          'text/plain',
          'application/pdf',
        ].includes(file.type) || file.name.endsWith('.docx') || file.name.endsWith('.doc'),
      'Chỉ chấp nhận file .doc, .docx, .txt, .pdf'
    )
    .refine((file) => file.size <= 10 * 1024 * 1024, 'File tối đa 10MB'),
});

export type DocumentUploadFormData = z.infer<typeof documentUploadSchema>;
