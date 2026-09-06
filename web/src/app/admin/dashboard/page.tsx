'use client';

import React from 'react';
import { useRouter } from 'next/navigation';
import { PartnerDashboardView } from '@/components/partner/PartnerDashboardView';

export default function DashboardPage() {
  const router = useRouter();

  const handleSelectTab = (tabKey: string) => {
    switch (tabKey) {
      case 'partner_dashboard':
        router.push('/admin/dashboard');
        break;
      case 'inbox':
        router.push('/admin/inbox');
        break;
      case 'customers':
        router.push('/admin/customers');
        break;
      case 'calls':
        router.push('/admin/calls');
        break;
      case 'learning':
        router.push('/admin/learning');
        break;
      case 'knowledge':
        router.push('/admin/knowledge');
        break;
      case 'partner_analytics':
        router.push('/admin/analytics');
        break;
      case 'permissions':
      case 'partner_config':
        router.push('/admin/permissions');
        break;
      case 'config':
        router.push('/admin/config');
        break;
      case 'test_data':
        router.push('/admin/test-data');
        break;
      default:
        router.push('/admin/' + tabKey);
    }
  };

  return <PartnerDashboardView onSelectTab={handleSelectTab} />;
}
