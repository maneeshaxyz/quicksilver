import { useState } from "react";
import EditIcon from "@mui/icons-material/Edit";
import MailOutlineIcon from "@mui/icons-material/MailOutline";
import { Fab } from "@mui/material";
import AppLayout from "../moles/AppLayout";
import ThreadList from "../moles/ThreadList";
import { useData } from "../../nonview/core/DataContext";
import { useCompose } from "../moles/ComposeProvider";

function MailPage() {
  const {
    emailThreads,
    loading,
    refreshActive,
    realtimeConnected,
    prefetchMessages,
  } = useData();
  const { openCompose } = useCompose();
  const [searchQuery, setSearchQuery] = useState("");

  const filteredThreads = emailThreads.filter((thread) => {
    if (!searchQuery) return true;
    const query = searchQuery.toLowerCase();
    return (
      thread.subject.toLowerCase().includes(query) ||
      thread.lastMessage.toLowerCase().includes(query) ||
      thread.participants.some((p) => p.name.toLowerCase().includes(query))
    );
  });

  return (
    <AppLayout
      title="Quicksilver"
      titleIcon={MailOutlineIcon}
      showSearch
      onSearch={setSearchQuery}
    >
      <ThreadList
        threads={filteredThreads}
        loading={loading}
        emptyMessage={
          searchQuery ? "No emails match your search" : "No emails yet"
        }
        onRefresh={searchQuery ? undefined : refreshActive}
        live={realtimeConnected}
        onPrefetch={prefetchMessages}
      />
      <Fab
        color="primary"
        aria-label="compose"
        onClick={() => openCompose()}
        sx={{
          position: "fixed",
          bottom: 24,
          right: 24,
        }}
      >
        <EditIcon />
      </Fab>
    </AppLayout>
  );
}

export default MailPage;
