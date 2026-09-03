export type UserRole = 'admin' | 'cskh' | 'customer';
export type SenderType = 'guest' | 'ai' | 'human_cs' | 'cs' | 'system';
export type CaseStatus = 'AI_ACTIVE' | 'NEEDS_HUMAN_CS' | 'HUMAN_CS_ACTIVE' | 'RESOLVED';
export type LearnStatus = 'PENDING' | 'APPROVED' | 'REJECTED';
export type CallStatus = 'RINGING' | 'ACTIVE' | 'ENDED' | 'MISSED' | 'REJECTED';

export type TabType =
  | 'inbox'
  | 'customers'
  | 'calls'
  | 'learning'
  | 'knowledge'
  | 'analytics'
  | 'config'
  | 'partner_dashboard'
  | 'partner_analytics'
  | 'partner_config'
  | 'update_data_test';

export interface User {
  id: number;
  username: string;
  full_name: string;
  role: UserRole;
  is_active: boolean;
}

export interface SessionUser {
  token: string;
  username: string;
  full_name: string;
  role: UserRole;
}

export interface GuestSession {
  guest_id: string;
  display_name: string;
  phone?: string;
  session_id: string;
  token: string;
}

export interface CustomerProfile {
  id: number;
  guest_id: string;
  display_name: string;
  phone: string;
  last_session_id?: string;
  last_message?: string;
  last_status?: string;
  created_at: string;
  updated_at: string;
}

export interface Message {
  id: number;
  session_id: string;
  sender_type: SenderType;
  sender_id: string;
  content: string;
  client_msg_id?: string;
  is_learned?: boolean;
  created_at: string;
}

export interface ChatCase {
  id: number;
  session_id: string;
  guest_id?: string;
  customer_name: string;
  customer_phone?: string;
  status: CaseStatus;
  assigned_cs: string;
  last_message: string;
  resolution_note: string;
  created_at: string;
  updated_at: string;
}

export interface LearningItem {
  id: number;
  session_id: string;
  question: string;
  answer: string;
  status: LearnStatus;
  created_by: string;
  approved_by?: string;
  created_at: string;
  approved_at?: string;
}

export interface QAPair {
  question: string;
  answer: string;
}

export interface AnalyticsStats {
  total_cases: number;
  total_sessions: number;
  ai_active_cases: number;
  needs_human_cases: number;
  active_human_cases: number;
  resolved_cases: number;
  ai_self_service_rate: number;
  ai_service_rate?: number;
  total_learned_qa: number;
  pending_learn_count: number;
}

export interface KnowledgeDocument {
  filename: string;
  size_kb: string;
}

export interface KnowledgeChunk {
  id: string | number;
  text: string;
  source: string;
}

export interface KnowledgeOverview {
  total_chunks: number;
  total_documents: number;
  collection_name?: string;
  documents: KnowledgeDocument[];
  chunks?: KnowledgeChunk[];
}

export interface SystemConfig {
  system_prompt: string;
  llm_model: string;
  temperature: number;
}

export type WSEventType =
  | 'message'
  | 'typing'
  | 'stop_typing'
  | 'unread'
  | 'case_update'
  | 'learning_update'
  | 'call_offer'
  | 'call_answer'
  | 'call_ice'
  | 'call_end'
  | 'call_ring'
  | 'ai_status';

export interface WSEvent<T = any> {
  type: WSEventType;
  session_id: string;
  payload: T;
  sender_id?: string;
  timestamp: string;
}

export interface SystemErrorItem {
  id: string;
  timestamp: string;
  source: string;
  title: string;
  details: string;
  severity: 'low' | 'medium' | 'high';
  isHandled: boolean;
  suggestedFix: string;
}

export type PermissionLevel = 'act' | 'view' | 'none';

export interface RolePermissionItem {
  id?: number;
  role_name: string;
  feature_key: string;
  permission_level: PermissionLevel;
  can_view: boolean;
  can_edit: boolean;
  updated_at?: string;
}
