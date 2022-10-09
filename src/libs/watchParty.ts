interface Member {
  id: string; // socket id
  name: string;
  isHost: boolean;
  isWatching: boolean; // keep track of whether the member is watching the stream
}

interface PlayerInfo {
  video: {
    torrentID: string;
    slug: string;
    title: string;
  };
  currentTime: number;
  totalDuration: number;
  isPlaying: boolean;
}

interface Party {
  id: string;
  slug: string;
  maxViewers: number;
  partyPlayerControl: boolean;
  host: string;
  members: Member[];
  playerInfo: PlayerInfo;
}

class WatchParty {
  public id: string;

  public slug: string;

  public maxViewers: number;

  public partyPlayerControl: boolean;

  public host: string;

  public members: Member[];

  public playerInfo: PlayerInfo;

  constructor(party: Party) {
    this.id = party.id;
    this.slug = party.slug;
    this.maxViewers = party.maxViewers;
    this.partyPlayerControl = party.partyPlayerControl;
    this.host = party.host;
    this.members = party.members;
    this.playerInfo = party.playerInfo;
  }

  public addMember(member: Member): void {
    this.members.push(member);
  }

  public removeMember(member: Member): void {
    this.members = this.members.filter(m => m.id !== member.id);
  }

  public getMember(memberId: string): Member | undefined {
    return this.members.find(m => m.id === memberId);
  }

  public allowPlayerControl(bool: boolean): void {
    this.partyPlayerControl = bool;
  }

  public setPlayerInfo(playerInfo: PlayerInfo): void {
    this.playerInfo = playerInfo;
  }

  public getPlayerInfo(): PlayerInfo {
    return this.playerInfo;
  }

  public setVideoCurrentTime(time: number): void {
    this.playerInfo.currentTime = time;
  }

  public getVideoCurrentTime(): number {
    return this.playerInfo.currentTime;
  }

  public setIsPlaying(bool: boolean): void {
    this.playerInfo.isPlaying = bool;
  }
}

export default WatchParty;
