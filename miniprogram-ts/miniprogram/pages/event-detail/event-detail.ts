import { DefineComponent, navigateTo, RootComponent } from "annil";

import { getEventDetail } from "../../api/events";
import type { EventDetail, GroupLevel } from "../../types/models";
import {
  danceTypeText,
  eventLevelText,
  eventStatusText,
  formatEventDate,
  formatEventDeadline,
  formatEventFee,
} from "../../utils/event";
import { formatDate } from "../../utils/format";
import type { $Register } from "../register/register";

interface TimelineView {
  label: string;
  dateText: string;
  done: boolean;
}

interface GroupItemView {
  groupId: number;
  danceTypeText: string;
  registered: number;
  quota: number;
  remaining: number;
  tagText: string;
  tagClass: string;
  percent: number;
  enabled: boolean;
}

interface GroupSectionView {
  level: GroupLevel;
  birthRange: string;
  items: GroupItemView[];
}

const LEVEL_ORDER: GroupLevel[] = ["U11", "U13", "U15"];

function timelineView(event: EventDetail): TimelineView[] {
  const nodes: Array<{ label: string; value: string | null }> = [
    { label: "报名开始", value: event.timeline.reg_start_at },
    { label: "截止报名", value: event.timeline.reg_end_at },
    { label: "线下抽签", value: event.timeline.lottery_at },
    { label: "核对音频", value: event.timeline.material_check_at },
    { label: "开始比赛", value: event.timeline.start_at },
  ];
  const now = Date.now();
  return nodes.map((node) => {
    const time = node.value ? new Date(node.value).getTime() : 0;
    return {
      label: node.label,
      dateText: node.value ? formatDate(node.value, "MM.DD") : "—",
      done: time > 0 && time <= now,
    };
  });
}

function groupSections(event: EventDetail): GroupSectionView[] {
  const byLevel = new Map<GroupLevel, GroupItemView[]>();
  const ranges = new Map<GroupLevel, string>();
  for (const group of event.groups) {
    const remaining = Math.max(0, group.remaining);
    const percent =
      group.quota > 0 ? Math.min(100, Math.round((group.registered / group.quota) * 100)) : 0;
    const item: GroupItemView = {
      groupId: group.group_id,
      danceTypeText: danceTypeText(group.dance_type),
      registered: group.registered,
      quota: group.quota,
      remaining,
      tagText: remaining === 0 ? "已满" : `剩余 ${remaining} 席`,
      tagClass: !group.enabled
        ? "app-tag--muted"
        : remaining === 0
          ? "app-tag--muted"
          : remaining <= 3
            ? "app-tag--yellow"
            : "",
      percent,
      enabled: group.enabled,
    };
    const items = byLevel.get(group.level) ?? [];
    items.push(item);
    byLevel.set(group.level, items);
    ranges.set(group.level, `${group.birth_year_from}–${group.birth_year_to} 年出生`);
  }
  return LEVEL_ORDER.filter((level) => byLevel.has(level)).map((level) => ({
    level,
    birthRange: ranges.get(level) ?? "",
    items: byLevel.get(level) ?? [],
  }));
}

const rootComponent = RootComponent()({
  isPage: true,
  properties: {
    eventId: { type: Number, value: 0 },
  },
  data: {
    event: null as EventDetail | null,
    loading: true,
    loadError: null as string | null,
    honorsOpen: false,
  },
  computed: {
    levelText(): string {
      return this.data.event ? eventLevelText(this.data.event.level) : "";
    },
    statusText(): string {
      return this.data.event ? eventStatusText(this.data.event.status) : "";
    },
    eventDateText(): string {
      return this.data.event ? formatEventDate(this.data.event.event_date) : "";
    },
    feeText(): string {
      return this.data.event ? formatEventFee(this.data.event.fee) : "";
    },
    cityVenueText(): string {
      if (!this.data.event) {
        return "";
      }
      return `${this.data.event.city || "城市待定"} · ${this.data.event.venue || "场馆待定"}`;
    },
    statusHint(): string {
      if (!this.data.event) {
        return "";
      }
      if (this.data.event.status === "registering") {
        return this.data.event.timeline.reg_end_at
          ? `截止报名：${formatEventDeadline(this.data.event.timeline.reg_end_at)}`
          : "报名进行中";
      }
      if (this.data.event.status === "upcoming") {
        return this.data.event.timeline.reg_start_at
          ? `${formatEventDeadline(this.data.event.timeline.reg_start_at)} 开始报名`
          : "暂未开始报名";
      }
      return "报名已截止";
    },
    canRegister(): boolean {
      return this.data.event?.status === "registering";
    },
    ctaText(): string {
      if (!this.data.event) {
        return "加载中…";
      }
      if (this.data.event.status === "registering") {
        return this.data.event.fee > 0
          ? `立即报名 ${formatEventFee(this.data.event.fee)}`
          : "立即报名";
      }
      if (this.data.event.status === "upcoming") {
        return "暂未开始报名";
      }
      return "报名已截止";
    },
    timelineItems(): TimelineView[] {
      return this.data.event ? timelineView(this.data.event) : [];
    },
    groupSections(): GroupSectionView[] {
      return this.data.event ? groupSections(this.data.event) : [];
    },
    rules(): Array<{ label: string; value: string }> {
      if (!this.data.event) {
        return [];
      }
      const items = [
        { label: "报名费用", value: formatEventFee(this.data.event.fee) },
        {
          label: "作品时长",
          value: this.data.event.work_duration_limit || "以现场要求为准",
        },
        {
          label: "比赛流程",
          value: this.data.event.process_desc || "以赛事通知为准",
        },
      ];
      return items;
    },
  },
  pageLifetimes: {
    onLoad(props) {
      if (!props.eventId || props.eventId <= 0) {
        this.setData({ loading: false, loadError: "赛事参数不正确" });
        return;
      }
      void this.loadEvent();
    },
  },
  methods: {
    async loadEvent(): Promise<void> {
      if (!this.data.eventId) {
        return;
      }
      this.setData({ loading: true, loadError: null });
      try {
        const event = await getEventDetail(this.data.eventId);
        this.setData({ event });
      } catch (error) {
        this.setData({ loadError: error instanceof Error ? error.message : "加载赛事失败" });
      } finally {
        this.setData({ loading: false });
      }
    },
    onRetry() {
      void this.loadEvent();
    },
    onHonorsTap() {
      this.setData({ honorsOpen: !this.data.honorsOpen });
    },
    onRegisterTap() {
      const event = this.data.event;
      if (!event || !this.data.canRegister) {
        return;
      }
      void navigateTo<$Register>({
        url: "/pages/register/register",
        data: { eventId: event.id },
      });
    },
  },
});

const eventDetail = DefineComponent({
  path: "/pages/event-detail/event-detail",
  rootComponent,
});

export type $EventDetail = typeof eventDetail;
