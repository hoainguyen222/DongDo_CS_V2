/**
 * Admin Store — Zustand
 * Quản lý trạng thái phức tạp của trang admin (case selection, modal, editing...)
 * Không chứa data từ API (đó là TanStack Query job)
 */

import { create } from 'zustand';
import type { ChatCase, CustomerProfile, QAPair } from '@/lib/types';

type TabType =
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

interface AdminState {
  // Active navigation tab
  activeTab: TabType;
  setActiveTab: (tab: TabType) => void;

  // Selected case (inbox)
  selectedCase: ChatCase | null;
  setSelectedCase: (c: ChatCase | null) => void;

  // Selected customer (customer management)
  selectedCustomer: CustomerProfile | null;
  setSelectedCustomer: (c: CustomerProfile | null) => void;

  // Resolve case modal
  showResolveModal: boolean;
  setShowResolveModal: (show: boolean) => void;
  modalQAPairs: QAPair[];
  setModalQAPairs: (pairs: QAPair[]) => void;
  modalEnableLearn: boolean;
  setModalEnableLearn: (v: boolean) => void;

  // Edit customer modal
  showEditCustomerModal: boolean;
  setShowEditCustomerModal: (show: boolean) => void;

  // Voice history modal
  showVoiceHistoryModal: boolean;
  setShowVoiceHistoryModal: (show: boolean) => void;

  // Editing learning item
  editingLearningId: number | null;
  setEditingLearningId: (id: number | null) => void;

  // Filter states
  caseFilter: string;
  setCaseFilter: (f: string) => void;
  customerSearch: string;
  setCustomerSearch: (s: string) => void;

  // Pagination state (for local pagination control)
  casePage: number;
  setCasePage: (p: number) => void;
  customerPage: number;
  setCustomerPage: (p: number) => void;
  learningPage: number;
  setLearningPage: (p: number) => void;
  casePageSize: number;
  setCasePageSize: (s: number) => void;
  customerPageSize: number;
  setCustomerPageSize: (s: number) => void;
  learningPageSize: number;
  setLearningPageSize: (s: number) => void;

  // Config form
  configPrompt: string;
  setConfigPrompt: (v: string) => void;
  configModel: string;
  setConfigModel: (v: string) => void;
  configTemp: number;
  setConfigTemp: (v: number) => void;

  // Reply text (inbox)
  replyText: string;
  setReplyText: (v: string) => void;
  // Resolution note
  resolveNote: string;
  setResolveNote: (v: string) => void;
}

export const useAdminStore = create<AdminState>((set) => ({
  activeTab: 'inbox',
  setActiveTab: (tab) => set({ activeTab: tab }),

  selectedCase: null,
  setSelectedCase: (c) => set({ selectedCase: c }),

  selectedCustomer: null,
  setSelectedCustomer: (c) => set({ selectedCustomer: c }),

  showResolveModal: false,
  setShowResolveModal: (show) => set({ showResolveModal: show }),
  modalQAPairs: [],
  setModalQAPairs: (pairs) => set({ modalQAPairs: pairs }),
  modalEnableLearn: true,
  setModalEnableLearn: (v) => set({ modalEnableLearn: v }),

  showEditCustomerModal: false,
  setShowEditCustomerModal: (show) => set({ showEditCustomerModal: show }),

  showVoiceHistoryModal: false,
  setShowVoiceHistoryModal: (show) => set({ showVoiceHistoryModal: show }),

  editingLearningId: null,
  setEditingLearningId: (id) => set({ editingLearningId: id }),

  caseFilter: '',
  setCaseFilter: (f) => set({ caseFilter: f }),
  customerSearch: '',
  setCustomerSearch: (s) => set({ customerSearch: s }),

  casePage: 1,
  setCasePage: (p) => set({ casePage: p }),
  customerPage: 1,
  setCustomerPage: (p) => set({ customerPage: p }),
  learningPage: 1,
  setLearningPage: (p) => set({ learningPage: p }),
  casePageSize: 10,
  setCasePageSize: (s) => set({ casePageSize: s }),
  customerPageSize: 10,
  setCustomerPageSize: (s) => set({ customerPageSize: s }),
  learningPageSize: 10,
  setLearningPageSize: (s) => set({ learningPageSize: s }),

  configPrompt: '',
  setConfigPrompt: (_v) => {},
  configModel: '',
  setConfigModel: (_v) => {},
  configTemp: 0.1,
  setConfigTemp: (_v) => {},

  replyText: '',
  setReplyText: (v) => set({ replyText: v }),
  resolveNote: '',
  setResolveNote: (v) => set({ resolveNote: v }),
}));
