import { DefineComponent, RootComponent, typeEqual } from "annil";

const rootComponent = RootComponent()({
  properties: {
    text: { type: String, value: "暂无数据" },
    showRetry: { type: Boolean, value: false },
    retryText: { type: String, value: "重新加载" },
  },
  customEvents: {
    retry: undefined,
  },
  methods: {
    onRetryTap() {
      this.retry();
    },
  },
});

const emptyState = DefineComponent({
  name: "emptyState",
  rootComponent,
});

/** 对外组件文档类型：父组件用 CustomComponent<Root, $EmptyState>() 接入。 */
export type $EmptyState = {
  properties: {
    emptyState_text?: string;
    emptyState_showRetry?: boolean;
    emptyState_retryText?: string;
  };
  events: {
    emptyState_retry: undefined;
  };
};

typeEqual<$EmptyState>()(emptyState);
